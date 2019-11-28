package scw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/scaleway/scaleway-sdk-go/internal/errors"
)

// ServiceInfo contains API metadata
// These metadata are only here for debugging. Do not rely on these values
type ServiceInfo struct {
	// Name is the name of the API
	Name string `json:"name"`

	// Description is a human readable description for the API
	Description string `json:"description"`

	// Version is the version of the API
	Version string `json:"version"`

	// DocumentationUrl is the a web url where the documentation of the API can be found
	DocumentationUrl *string `json:"documentation_url"`
}

// File is the structure used to receive / send a file from / to the API
type File struct {
	// Name of the file
	Name string `json:"name"`

	// ContentType used in the HTTP header `Content-Type`
	ContentType string `json:"content_type"`

	// Content of the file
	Content io.Reader `json:"content"`
}

func (f *File) UnmarshalJSON(b []byte) error {
	type file File
	var tmpFile struct {
		file
		Content []byte `json:"content"`
	}

	err := json.Unmarshal(b, &tmpFile)
	if err != nil {
		return err
	}

	tmpFile.file.Content = bytes.NewReader(tmpFile.Content)

	*f = File(tmpFile.file)
	return nil
}

// Money represents an amount of money with its currency type.
type Money struct {
	// CurrencyCode is the 3-letter currency code defined in ISO 4217.
	CurrencyCode string `json:"currency_code,omitempty"`

	// Units is the whole units of the amount.
	// For example if `currencyCode` is `"USD"`, then 1 unit is one US dollar.
	Units int64 `json:"units,omitempty"`

	// Nanos is the number of nano (10^-9) units of the amount.
	// The value must be between -999,999,999 and +999,999,999 inclusive.
	// If `units` is positive, `nanos` must be positive or zero.
	// If `units` is zero, `nanos` can be positive, zero, or negative.
	// If `units` is negative, `nanos` must be negative or zero.
	// For example $-1.75 is represented as `units`=-1 and `nanos`=-750,000,000.
	Nanos int32 `json:"nanos,omitempty"`
}

// NewMoneyFromFloat conerts a float with currency to a Money object.
func NewMoneyFromFloat(value float64, currency string) *Money {
	return &Money{
		CurrencyCode: currency,
		Units:        int64(value),
		Nanos:        int32((value - float64(int64(value))) * 1000000000),
	}
}

// ToFloat converts a Money object to a float.
func (m *Money) ToFloat() float64 {
	return float64(m.Units) + float64(m.Nanos)/1000000000
}

// Money represents a size in bytes.
type Size uint64

const (
	B  Size = 1
	KB      = 1000 * B
	MB      = 1000 * KB
	GB      = 1000 * MB
	TB      = 1000 * GB
	PB      = 1000 * TB
)

// String returns the string representation of a Size.
func (s Size) String() string {
	return fmt.Sprintf("%d", s)
}

// TimeSeries represents a time series that could be used for graph purposes.
type TimeSeries struct {
	// Name of the metric.
	Name string `json:"name"`

	// Points contains all the points that composed the series.
	Points []*TimeSeriesPoint `json:"points"`

	// Metadata contains some string metadata related to a metric.
	Metadata map[string]string `json:"metadata"`
}

// TimeSeriesPoint represents a point of a time series.
type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float32
}

func (tsp *TimeSeriesPoint) MarshalJSON() ([]byte, error) {
	timestamp := tsp.Timestamp.Format(time.RFC3339)
	value, err := json.Marshal(tsp.Value)
	if err != nil {
		return nil, err
	}

	return []byte(`["` + timestamp + `",` + string(value) + "]"), nil
}

func (tsp *TimeSeriesPoint) UnmarshalJSON(b []byte) error {
	point := [2]interface{}{}

	err := json.Unmarshal(b, &point)
	if err != nil {
		return err
	}

	if len(point) != 2 {
		return errors.New("invalid point array")
	}

	strTimestamp, isStrTimestamp := point[0].(string)
	if !isStrTimestamp {
		return errors.New("%s timestamp is not a string in RFC 3339 format", point[0])
	}
	timestamp, err := time.Parse(time.RFC3339, strTimestamp)
	if err != nil {
		return errors.New("%s timestamp is not in RFC 3339 format", point[0])
	}
	tsp.Timestamp = timestamp

	// By default, JSON unmarshal a float in float64 but the TimeSeriesPoint is a float32 value.
	value, isValue := point[1].(float64)
	if !isValue {
		return errors.New("%s is not a valid float32 value", point[1])
	}
	tsp.Value = float32(value)

	return nil
}
