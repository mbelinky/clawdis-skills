# tuya-hub

Go CLI for controlling Tuya devices via Tuya Cloud or local Home Assistant.

## Build

```bash
go build -o bin/tuya ./cmd/tuya
```

## Quick start

```bash
./bin/tuya config --backend cloud
./bin/tuya discover --backend cloud --json
./bin/tuya poll --backend cloud --kind temperature
```

## Config

Default path: `~/.config/tuya-hub/config.yaml`

Cloud example:
```yaml
backend: cloud
cloud:
  accessId: "YOUR_TUYA_ACCESS_ID"
  accessKey: "YOUR_TUYA_ACCESS_KEY"
  endpoint: "https://openapi.tuyaeu.com"
  schema: ""  # optional; for user lookup
  userId: ""  # must be UID from Link Tuya App Account
```

## Notes

- Temperature values are auto-scaled when tenths are detected; raw value is returned as `value_raw` in JSON output.
- `permission deny` almost always means the app account UID is not linked to the project or region mismatch.

## Commands

```bash
./bin/tuya config --backend cloud
./bin/tuya users --try-common
./bin/tuya discover --backend cloud
./bin/tuya poll --backend cloud --kind temperature
./bin/tuya get --backend cloud --id <device_id>
./bin/tuya set --backend cloud --id <device_id> --code switch_1 --value true
```
