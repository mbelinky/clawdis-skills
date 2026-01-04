package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"tuya-hub/internal/cloud"
	"tuya-hub/internal/config"
	"tuya-hub/internal/ha"
	"tuya-hub/internal/util"
)

const version = "0.3.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "discover":
		runDiscover(os.Args[2:])
	case "devices":
		runDiscover(os.Args[2:])
	case "poll":
		runPoll(os.Args[2:])
	case "get":
		runGet(os.Args[2:])
	case "set":
		runSet(os.Args[2:])
	case "call":
		runCall(os.Args[2:])
	case "users":
		runUsers(os.Args[2:])
	case "config":
		runConfig(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("tuya-hub CLI (Go)")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  tuya config [--backend cloud|ha] [--config <path>]")
	fmt.Println("  tuya users --schema <schema> [--try-common] [--json]")
	fmt.Println("  tuya discover [--backend ha|cloud] [--filter <text>] [--json]")
	fmt.Println("  tuya devices [--backend ha|cloud] [--filter <text>] [--json]")
	fmt.Println("  tuya poll --kind temperature|humidity [--backend ha|cloud] [--json]")
	fmt.Println("  tuya get --entity <entity_id> [--json]")
	fmt.Println("  tuya get --backend cloud --id <device_id> [--code <status_code>] [--json]")
	fmt.Println("  tuya set --entity <entity_id> --state on|off")
	fmt.Println("  tuya set --backend cloud --id <device_id> --code <command_code> --value <json>")
	fmt.Println("  tuya call --service <domain.service> [--data <json>] [--json]")
	fmt.Println("  tuya version")
	fmt.Println("")
	fmt.Println("Config:")
	fmt.Println("  - default: ~/.config/tuya-hub/config.yaml")
	fmt.Println("  - env: TUYA_BACKEND, TUYA_HA_URL, TUYA_HA_TOKEN,")
	fmt.Println("         TUYA_CLOUD_ACCESS_ID, TUYA_CLOUD_ACCESS_KEY,")
	fmt.Println("         TUYA_CLOUD_ENDPOINT, TUYA_CLOUD_SCHEMA, TUYA_CLOUD_USER_ID")
}

func loadConfig(path, backendOverride string) (*config.Config, string) {
	cfg, err := config.Load(path)
	if err != nil {
		fatal(err)
	}
	cfg.ApplyEnv()
	backend := backendOverride
	if backend == "" {
		backend = cfg.BackendOr("ha")
	}
	if err := cfg.Validate(backend); err != nil {
		fatal(err)
	}
	return cfg, backend
}

func haClient(cfg *config.Config) *ha.Client {
	return ha.New(cfg.HomeAssistant.URL, cfg.HomeAssistant.Token)
}

func cloudClient(cfg *config.Config) *cloud.Client {
	return cloud.New(cfg.Cloud.Endpoint, cfg.Cloud.AccessID, cfg.Cloud.AccessKey, cfg.Cloud.UserID)
}

func runDiscover(args []string) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	configPath := fs.String("config", "", "config path")
	backend := fs.String("backend", "", "backend (ha|cloud)")
	filter := fs.String("filter", "", "filter substring")
	jsonOut := fs.Bool("json", false, "json output")
	fs.Parse(args)

	cfg, be := loadConfig(*configPath, *backend)
	switch be {
	case "ha":
		client := haClient(cfg)
		states, err := client.States()
		if err != nil {
			fatal(err)
		}

		filtered := filterStates(states, *filter)
		if *jsonOut {
			writeJSON(filtered)
			return
		}

		fmt.Printf("%-40s %-12s %s\n", "ENTITY", "STATE", "FRIENDLY_NAME")
		for _, st := range filtered {
			name, _ := st.Attributes["friendly_name"].(string)
			fmt.Printf("%-40s %-12s %s\n", st.EntityID, st.State, name)
		}
	case "cloud":
		client := cloudClient(cfg)
		devices, err := client.GetDevices()
		if err != nil {
			fatal(err)
		}
		filtered := filterCloudDevices(devices, *filter)
		if *jsonOut {
			writeJSON(filtered)
			return
		}
		fmt.Printf("%-30s %-30s %-12s %s\n", "DEVICE_ID", "NAME", "CATEGORY", "ONLINE")
		for _, dev := range filtered {
			fmt.Printf("%-30s %-30s %-12s %v\n", dev.ID, dev.Name, dev.Category, dev.Online)
		}
	default:
		fatal(fmt.Errorf("discover not implemented for backend %s", be))
	}
}

type cloudReading struct {
	DeviceID string      `json:"deviceId"`
	Name     string      `json:"name"`
	Code     string      `json:"code"`
	Value    interface{} `json:"value"`
	Raw      interface{} `json:"value_raw,omitempty"`
}

func runPoll(args []string) {
	fs := flag.NewFlagSet("poll", flag.ExitOnError)
	configPath := fs.String("config", "", "config path")
	backend := fs.String("backend", "", "backend (ha|cloud)")
	kind := fs.String("kind", "temperature", "temperature|humidity")
	jsonOut := fs.Bool("json", false, "json output")
	fs.Parse(args)

	cfg, be := loadConfig(*configPath, *backend)
	switch be {
	case "ha":
		client := haClient(cfg)
		states, err := client.States()
		if err != nil {
			fatal(err)
		}

		filtered := filterByKind(states, *kind)
		if *jsonOut {
			writeJSON(filtered)
			return
		}

		fmt.Printf("%-40s %-12s %s\n", "ENTITY", "STATE", "UNIT")
		for _, st := range filtered {
			unit, _ := st.Attributes["unit_of_measurement"].(string)
			fmt.Printf("%-40s %-12s %s\n", st.EntityID, st.State, unit)
		}
	case "cloud":
		client := cloudClient(cfg)
		devices, err := client.GetDevices()
		if err != nil {
			fatal(err)
		}
		readings := make([]cloudReading, 0)
		for _, dev := range devices {
			statuses, err := client.GetDeviceStatus(dev.ID)
			if err != nil {
				fatal(err)
			}
			for _, st := range filterCloudStatuses(statuses, *kind) {
				val, raw := scaleCloudValue(st.Code, st.Value)
				readings = append(readings, cloudReading{
					DeviceID: dev.ID,
					Name:     dev.Name,
					Code:     st.Code,
					Value:    val,
					Raw:      raw,
				})
			}
		}
		sort.Slice(readings, func(i, j int) bool {
			if readings[i].DeviceID == readings[j].DeviceID {
				return readings[i].Code < readings[j].Code
			}
			return readings[i].DeviceID < readings[j].DeviceID
		})
		if *jsonOut {
			writeJSON(readings)
			return
		}
		fmt.Printf("%-30s %-30s %-20s %s\n", "DEVICE_ID", "NAME", "CODE", "VALUE")
		for _, r := range readings {
			val := fmt.Sprintf("%v", r.Value)
			if r.Raw != nil {
				val = fmt.Sprintf("%v (raw %v)", r.Value, r.Raw)
			}
			fmt.Printf("%-30s %-30s %-20s %s\n", r.DeviceID, r.Name, r.Code, val)
		}
	default:
		fatal(fmt.Errorf("poll not implemented for backend %s", be))
	}
}

func runGet(args []string) {
	fs := flag.NewFlagSet("get", flag.ExitOnError)
	configPath := fs.String("config", "", "config path")
	backend := fs.String("backend", "", "backend (ha|cloud)")
	entity := fs.String("entity", "", "entity id")
	deviceID := fs.String("id", "", "device id (cloud)")
	code := fs.String("code", "", "status code (cloud)")
	jsonOut := fs.Bool("json", false, "json output")
	fs.Parse(args)

	cfg, be := loadConfig(*configPath, *backend)
	switch be {
	case "ha":
		if strings.TrimSpace(*entity) == "" {
			fatal(fmt.Errorf("--entity required"))
		}
		client := haClient(cfg)
		st, err := client.State(*entity)
		if err != nil {
			fatal(err)
		}
		if *jsonOut {
			writeJSON(st)
			return
		}
		fmt.Printf("%s = %s\n", st.EntityID, st.State)
	case "cloud":
		id := strings.TrimSpace(*deviceID)
		if id == "" {
			id = strings.TrimSpace(*entity)
		}
		if id == "" {
			fatal(fmt.Errorf("--id required for cloud backend"))
		}
		client := cloudClient(cfg)
		statuses, err := client.GetDeviceStatus(id)
		if err != nil {
			fatal(err)
		}
		if strings.TrimSpace(*code) != "" {
			for _, st := range statuses {
				if st.Code == *code {
					if *jsonOut {
						writeJSON(st)
						return
					}
					fmt.Printf("%s %s = %v\n", id, st.Code, st.Value)
					return
				}
			}
			fatal(fmt.Errorf("status code not found: %s", *code))
		}
		if *jsonOut {
			writeJSON(statuses)
			return
		}
		for _, st := range statuses {
			fmt.Printf("%s %s = %v\n", id, st.Code, st.Value)
		}
	default:
		fatal(fmt.Errorf("get not implemented for backend %s", be))
	}
}

func runSet(args []string) {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	configPath := fs.String("config", "", "config path")
	backend := fs.String("backend", "", "backend (ha|cloud)")
	entity := fs.String("entity", "", "entity id")
	deviceID := fs.String("id", "", "device id (cloud)")
	state := fs.String("state", "", "on|off")
	code := fs.String("code", "", "command code (cloud)")
	value := fs.String("value", "", "command value (cloud; json)")
	jsonOut := fs.Bool("json", false, "json output")
	fs.Parse(args)

	cfg, be := loadConfig(*configPath, *backend)
	switch be {
	case "ha":
		if strings.TrimSpace(*entity) == "" {
			fatal(fmt.Errorf("--entity required"))
		}
		if *state == "" {
			fatal(fmt.Errorf("--state required (on|off)"))
		}

		domain := ha.DomainFromEntity(*entity)
		if domain == "" {
			fatal(fmt.Errorf("could not infer domain from entity id"))
		}

		service := "turn_off"
		if strings.ToLower(*state) == "on" {
			service = "turn_on"
		}

		payload := map[string]any{"entity_id": *entity}
		client := haClient(cfg)
		res, err := client.CallService(domain, service, payload)
		if err != nil {
			fatal(err)
		}
		if *jsonOut {
			writeJSON(res)
			return
		}
		fmt.Printf("%s %s\n", *entity, service)
	case "cloud":
		id := strings.TrimSpace(*deviceID)
		if id == "" {
			id = strings.TrimSpace(*entity)
		}
		if id == "" {
			fatal(fmt.Errorf("--id required for cloud backend"))
		}
		if strings.TrimSpace(*code) == "" {
			fatal(fmt.Errorf("--code required for cloud backend"))
		}
		if strings.TrimSpace(*value) == "" {
			fatal(fmt.Errorf("--value required for cloud backend"))
		}
		v, err := util.ParseJSONValue(*value)
		if err != nil {
			fatal(err)
		}
		client := cloudClient(cfg)
		res, err := client.SendCommands(id, []map[string]any{{"code": *code, "value": v}})
		if err != nil {
			fatal(err)
		}
		if *jsonOut {
			writeJSON(res)
			return
		}
		fmt.Printf("sent %s %s\n", id, *code)
	default:
		fatal(fmt.Errorf("set not implemented for backend %s", be))
	}
}

func runCall(args []string) {
	fs := flag.NewFlagSet("call", flag.ExitOnError)
	configPath := fs.String("config", "", "config path")
	backend := fs.String("backend", "", "backend (ha|cloud)")
	service := fs.String("service", "", "domain.service")
	data := fs.String("data", "", "json data payload")
	jsonOut := fs.Bool("json", false, "json output")
	fs.Parse(args)

	if strings.TrimSpace(*service) == "" {
		fatal(fmt.Errorf("--service required"))
	}
	parts := strings.SplitN(*service, ".", 2)
	if len(parts) != 2 {
		fatal(fmt.Errorf("--service must be domain.service"))
	}

	cfg, be := loadConfig(*configPath, *backend)
	if be != "ha" {
		fatal(fmt.Errorf("call not implemented for backend %s", be))
	}

	payload, err := util.ParseJSONMap(*data)
	if err != nil {
		fatal(err)
	}

	client := haClient(cfg)
	res, err := client.CallService(parts[0], parts[1], payload)
	if err != nil {
		fatal(err)
	}
	if *jsonOut {
		writeJSON(res)
		return
	}
	fmt.Printf("called %s\n", *service)
}

func runUsers(args []string) {
	fs := flag.NewFlagSet("users", flag.ExitOnError)
	configPath := fs.String("config", "", "config path")
	schema := fs.String("schema", "", "app schema")
	tryCommon := fs.Bool("try-common", false, "try common schemas")
	sinceDays := fs.Int("since-days", 30, "days back for user lookup")
	pageSize := fs.Int("page-size", 100, "page size")
	pageNo := fs.Int("page", 1, "page number")
	jsonOut := fs.Bool("json", false, "json output")
	fs.Parse(args)

	cfg, _ := loadConfig(*configPath, "cloud")
	client := cloudClient(cfg)

	schemas := []string{}
	if strings.TrimSpace(*schema) != "" {
		schemas = []string{strings.TrimSpace(*schema)}
	} else if strings.TrimSpace(cfg.Cloud.Schema) != "" {
		schemas = []string{strings.TrimSpace(cfg.Cloud.Schema)}
	} else if *tryCommon {
		schemas = []string{"smartlife", "tuyaSmart", "smart_life", "tuya", "SmartLife"}
	} else {
		fatal(fmt.Errorf("schema required (set cloud.schema or --schema, or use --try-common)"))
	}

	if *sinceDays <= 0 {
		*sinceDays = 30
	}
	endTime := time.Now().Unix()
	startTime := endTime - int64(*sinceDays)*24*60*60

	var lastErr error
	for _, sc := range schemas {
		res, err := client.GetUsers(sc, *pageNo, *pageSize, startTime, endTime)
		if err != nil {
			lastErr = err
			continue
		}
		if *jsonOut {
			writeJSON(res)
			return
		}
		fmt.Printf("schema: %s (total: %d)\n", sc, res.Total)
		if len(res.List) == 0 {
			fmt.Println("(no users returned)")
			return
		}
		fmt.Printf("%-4s %-24s %-14s %s\n", "#", "USERNAME", "COUNTRY", "UID")
		for i, u := range res.List {
			fmt.Printf("%-4d %-24s %-14s %s\n", i+1, u.Username, u.CountryCode, u.UID)
		}
		return
	}

	if lastErr != nil {
		fatal(lastErr)
	}
	fatal(fmt.Errorf("no schemas succeeded"))
}

func runConfig(args []string) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	configPath := fs.String("config", "", "config path")
	backend := fs.String("backend", "", "backend (ha|cloud)")
	fs.Parse(args)

	existing, err := config.Load(*configPath)
	if err != nil {
		fatal(err)
	}
	if existing == nil {
		existing = &config.Config{}
	}
	cfg := *existing

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Tuya config wizard")
	fmt.Println("")

	be := strings.ToLower(strings.TrimSpace(*backend))
	if be == "" {
		be = strings.ToLower(promptDefault(reader, "Backend (cloud/ha)", cfg.BackendOr("cloud")))
	}
	if be != "cloud" && be != "ha" {
		fatal(fmt.Errorf("unknown backend: %s", be))
	}
	cfg.Backend = be

	if be == "cloud" {
		printCloudSteps()
		cfg.Cloud.AccessID = promptRequired(reader, "Tuya Access ID", cfg.Cloud.AccessID)
		cfg.Cloud.AccessKey = promptRequired(reader, "Tuya Access Key", cfg.Cloud.AccessKey)
		cfg.Cloud.Endpoint = promptEndpoint(reader, cfg.Cloud.Endpoint)
		cfg.Cloud.Region = promptDefault(reader, "Region label (optional)", inferRegion(cfg.Cloud.Endpoint, cfg.Cloud.Region))
		cfg.Cloud.Schema = promptDefault(reader, "App schema (optional; for user lookup)", cfg.Cloud.Schema)

		if promptYesNo(reader, "Test Tuya Cloud token now", true) {
			client := cloudClient(&cfg)
			tok, err := client.GetToken()
			if err != nil {
				fmt.Printf("Token test failed: %v\n", err)
			} else {
				uid := strings.TrimSpace(tok.UID)
				if uid == "" {
					fmt.Println("Token OK. UID not returned.")
				} else {
					fmt.Printf("Token OK. UID: %s\n", uid)
					if strings.TrimSpace(cfg.Cloud.UserID) == "" && promptYesNo(reader, "Use token UID as userId", false) {
						cfg.Cloud.UserID = uid
					}
				}
			}
		}

		if strings.TrimSpace(cfg.Cloud.Schema) != "" && promptYesNo(reader, "Lookup linked app users now", true) {
			client := cloudClient(&cfg)
			res, err := client.GetUsers(cfg.Cloud.Schema, 1, 100, 0, 0)
			if err != nil {
				fmt.Printf("User lookup failed: %v\n", err)
			} else if len(res.List) == 0 {
				fmt.Println("No users returned for schema.")
			} else {
				fmt.Printf("%-4s %-24s %-14s %s\n", "#", "USERNAME", "COUNTRY", "UID")
				for i, u := range res.List {
					fmt.Printf("%-4d %-24s %-14s %s\n", i+1, u.Username, u.CountryCode, u.UID)
				}
				if len(res.List) == 1 && strings.TrimSpace(cfg.Cloud.UserID) == "" {
					if promptYesNo(reader, "Use this UID as userId", true) {
						cfg.Cloud.UserID = res.List[0].UID
					}
				} else {
					sel := strings.TrimSpace(promptDefault(reader, "Pick user by number or paste uid (optional)", ""))
					if sel != "" {
						if idx, err := strconv.Atoi(sel); err == nil && idx >= 1 && idx <= len(res.List) {
							cfg.Cloud.UserID = res.List[idx-1].UID
						} else {
							cfg.Cloud.UserID = sel
						}
					}
				}
			}
		}

		if strings.TrimSpace(cfg.Cloud.UserID) == "" {
			cfg.Cloud.UserID = promptDefault(reader, "Tuya User ID (optional)", cfg.Cloud.UserID)
		}
	}

	if be == "ha" {
		cfg.HomeAssistant.URL = promptRequired(reader, "Home Assistant URL", cfg.HomeAssistant.URL)
		cfg.HomeAssistant.Token = promptRequired(reader, "Home Assistant token", cfg.HomeAssistant.Token)
	}

	if err := cfg.Validate(be); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	path, err := config.Save(*configPath, &cfg)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("Wrote %s\n", path)
}

func filterStates(states []ha.State, filter string) []ha.State {
	if strings.TrimSpace(filter) == "" {
		return sortStates(states)
	}
	needle := strings.ToLower(filter)
	out := make([]ha.State, 0, len(states))
	for _, st := range states {
		name, _ := st.Attributes["friendly_name"].(string)
		if strings.Contains(strings.ToLower(st.EntityID), needle) || strings.Contains(strings.ToLower(name), needle) {
			out = append(out, st)
		}
	}
	return sortStates(out)
}

func filterByKind(states []ha.State, kind string) []ha.State {
	kind = strings.ToLower(kind)
	out := make([]ha.State, 0, len(states))
	for _, st := range states {
		dc, _ := st.Attributes["device_class"].(string)
		unit, _ := st.Attributes["unit_of_measurement"].(string)
		if dc == kind {
			out = append(out, st)
			continue
		}
		if kind == "temperature" && strings.Contains(unit, "°") {
			out = append(out, st)
			continue
		}
		if kind == "humidity" && strings.Contains(strings.ToLower(unit), "%") {
			out = append(out, st)
			continue
		}
	}
	return sortStates(out)
}

func filterCloudDevices(devices []cloud.Device, filter string) []cloud.Device {
	if strings.TrimSpace(filter) == "" {
		return sortCloudDevices(devices)
	}
	needle := strings.ToLower(filter)
	out := make([]cloud.Device, 0, len(devices))
	for _, dev := range devices {
		if strings.Contains(strings.ToLower(dev.ID), needle) || strings.Contains(strings.ToLower(dev.Name), needle) {
			out = append(out, dev)
		}
	}
	return sortCloudDevices(out)
}

func filterCloudStatuses(statuses []cloud.Status, kind string) []cloud.Status {
	if strings.TrimSpace(kind) == "" {
		return statuses
	}
	kind = strings.ToLower(kind)
	out := make([]cloud.Status, 0, len(statuses))
	for _, st := range statuses {
		code := strings.ToLower(st.Code)
		switch kind {
		case "temperature":
			if strings.Contains(code, "temp") || strings.Contains(code, "temperature") {
				out = append(out, st)
			}
		case "humidity":
			if strings.Contains(code, "hum") {
				out = append(out, st)
			}
		default:
			if strings.Contains(code, kind) {
				out = append(out, st)
			}
		}
	}
	return out
}

func sortStates(states []ha.State) []ha.State {
	out := append([]ha.State(nil), states...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].EntityID < out[j].EntityID
	})
	return out
}

func scaleCloudValue(code string, value interface{}) (interface{}, interface{}) {
	codeLower := strings.ToLower(code)
	if strings.Contains(codeLower, "_f") {
		return value, nil
	}
	if !strings.Contains(codeLower, "temp") && !strings.Contains(codeLower, "temperature") {
		return value, nil
	}
	f, ok := toFloat(value)
	if !ok {
		return value, nil
	}
	if math.Abs(f) >= 50 {
		scaled := round1(f / 10.0)
		return scaled, value
	}
	return value, nil
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func sortCloudDevices(devices []cloud.Device) []cloud.Device {
	out := append([]cloud.Device(nil), devices...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].ID < out[j].ID
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func writeJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(data))
}

func promptDefault(reader *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, _ := reader.ReadString(10)
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptRequired(reader *bufio.Reader, label, def string) string {
	for {
		val := promptDefault(reader, label, def)
		if strings.TrimSpace(val) != "" {
			return val
		}
		fmt.Println("Value required.")
	}
}

func promptYesNo(reader *bufio.Reader, label string, def bool) bool {
	defStr := "y"
	if !def {
		defStr = "n"
	}
	for {
		val := strings.ToLower(strings.TrimSpace(promptDefault(reader, label+" (y/n)", defStr)))
		if val == "y" || val == "yes" {
			return true
		}
		if val == "n" || val == "no" {
			return false
		}
		fmt.Println("Please enter y or n.")
	}
}

func printCloudSteps() {
	fmt.Println("Tuya Cloud setup steps (high level):")
	fmt.Println("1) Create or open a Tuya Cloud project in the Tuya IoT platform.")
	fmt.Println("2) Note the Access ID and Access Key on the project Overview.")
	fmt.Println("3) Link your Tuya app account under Devices -> Link Tuya App Account.")
	fmt.Println("4) Use the UID from that link screen as userId (not the token uid).")
	fmt.Println("5) Pick the correct data center endpoint for your region.")
	fmt.Println("")
	fmt.Println("If you see permission deny, the app account/UID is not linked to this project.")
	fmt.Println("")
}

func promptEndpoint(reader *bufio.Reader, current string) string {
	fmt.Println("Choose a Tuya Cloud data center endpoint:")
	fmt.Println("  1) https://openapi.tuyaus.com   (Western America)")
	fmt.Println("  2) https://openapi-ueaz.tuyaus.com (Eastern America)")
	fmt.Println("  3) https://openapi.tuyaeu.com   (Central Europe)")
	fmt.Println("  4) https://openapi-weaz.tuyaeu.com (Western Europe)")
	fmt.Println("  5) https://openapi.tuyacn.com   (China)")
	fmt.Println("  6) https://openapi.tuyain.com   (India)")
	fmt.Println("  7) https://openapi-sg.iotbing.com (Singapore)")
	fmt.Println("")
	choice := promptDefault(reader, "Pick 1-7 or paste endpoint", current)
	choice = strings.TrimSpace(choice)
	switch choice {
	case "1":
		return "https://openapi.tuyaus.com"
	case "2":
		return "https://openapi-ueaz.tuyaus.com"
	case "3":
		return "https://openapi.tuyaeu.com"
	case "4":
		return "https://openapi-weaz.tuyaeu.com"
	case "5":
		return "https://openapi.tuyacn.com"
	case "6":
		return "https://openapi.tuyain.com"
	case "7":
		return "https://openapi-sg.iotbing.com"
	}
	if strings.TrimSpace(choice) == "" && current != "" {
		return current
	}
	return choice
}

func inferRegion(endpoint, fallback string) string {
	if fallback != "" {
		return fallback
	}
	if strings.Contains(endpoint, "tuyaus") {
		if strings.Contains(endpoint, "ueaz") {
			return "us-east"
		}
		return "us"
	}
	if strings.Contains(endpoint, "tuyaeu") {
		if strings.Contains(endpoint, "weaz") {
			return "eu-west"
		}
		return "eu"
	}
	if strings.Contains(endpoint, "tuyain") {
		return "in"
	}
	if strings.Contains(endpoint, "tuyacn") {
		return "cn"
	}
	if strings.Contains(endpoint, "iotbing") {
		return "sg"
	}
	return ""
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
