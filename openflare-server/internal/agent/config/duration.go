package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type MillisecondDuration time.Duration

func (d MillisecondDuration) Duration() time.Duration {
	return time.Duration(d)
}

func (d MillisecondDuration) String() string {
	return time.Duration(d).String()
}

func (d *MillisecondDuration) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*d = 0
		return nil
	}
	if strings.HasPrefix(raw, "\"") {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			*d = 0
			return nil
		}
		parsed, err := time.ParseDuration(text)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", text, err)
		}
		*d = MillisecondDuration(parsed)
		return nil
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid duration milliseconds %q: %w", raw, err)
	}
	*d = MillisecondDuration(time.Duration(ms) * time.Millisecond)
	return nil
}

func (d MillisecondDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).Milliseconds())
}
