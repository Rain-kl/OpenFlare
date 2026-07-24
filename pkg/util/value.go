package util

import "fmt"

// Interface2String converts a string, int, or float64 value to its string representation.
func Interface2String(inter interface{}) string {
	switch v := inter.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%f", v)
	}
	return "Not Implemented"
}
