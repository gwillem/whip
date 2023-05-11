package whip

import "encoding/json"

func (j *Job) ToJSON() ([]byte, error) {
	return json.Marshal(j)
}
