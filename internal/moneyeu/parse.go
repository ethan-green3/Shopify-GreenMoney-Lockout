package moneyeu

import (
	"encoding/json"
	"fmt"
)

func (r *CreateOrderExtResponse) FirstContent() (*CreateOrderExtContent, error) {
	if len(r.Content) == 0 {
		return nil, fmt.Errorf("empty content")
	}

	// Try object first
	var obj CreateOrderExtContent
	if err := json.Unmarshal(r.Content, &obj); err == nil && (obj.ID != 0 || obj.Url != "") {
		return &obj, nil
	}

	// Try array
	var arr []CreateOrderExtContent
	if err := json.Unmarshal(r.Content, &arr); err == nil {
		if len(arr) == 0 {
			return nil, fmt.Errorf("content array is empty")
		}
		return &arr[0], nil
	}

	return nil, fmt.Errorf("content is neither object nor array: %s", string(r.Content))
}
