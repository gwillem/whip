package whip

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

func LoadPlaybook(path string) (*Playbook, error) {
	rawData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var anyMap interface{}
	if e := yaml.Unmarshal(rawData, &anyMap); e != nil {
		return nil, e
	}

	return yamlToPlaybook(anyMap)
}

func yamlToPlaybook(y any) (*Playbook, error) {
	pb := Playbook{}
	md := mapstructure.Metadata{}

	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &pb,
		Metadata:         &md,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			parseTasksFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(y)
	if err != nil {
		return nil, err
	}

	// TODO log to debug
	// fmt.Println("unused:", md.Unused)
	// pp.Println(pb)
	// fmt.Println("mapstruct decode succeeded")
	return &pb, nil
}

func parseTasksFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		// return func(t reflect.Value, f reflect.Value, data any) (any, error) {
		if t != reflect.TypeOf(Task{}) {
			return data, nil
		}
		if f != reflect.TypeOf(map[string]any{}) {
			return nil, fmt.Errorf("expected map[string]any{}, got %v", f)
		}

		// fmt.Println("parseTasks")
		// fmt.Println("t:", t)
		// fmt.Println("f:", f)
		// pp.Println(data)

		for k, v := range data.(map[string]any) {
			// fmt.Println("k:", k)
			// fmt.Println("v:", v)

			// TODO import this from runners pkg
			if slices.Contains([]string{"authorized_key", "shell", "command"}, k) {
				delete(data.(map[string]any), k)
				data.(map[string]any)["runner"] = k
				switch v.(type) {
				case string:
					data.(map[string]any)["args"] = parseArgString(v.(string))
				case map[string]any:
					data.(map[string]any)["args"] = v
				default:
					return nil, fmt.Errorf("unexpected type for task arg: %v", v)
				}
				continue
			}
		}
		// fmt.Println("data:", data)
		return data, nil
	}
}

func parseArgString(arg string) map[string]string {
	kv := map[string]string{}

	baseArgs := []string{}
	for _, t := range strings.Split(arg, " ") {
		if strings.Contains(t, "=") {
			opt := strings.SplitN(t, "=", 2)

			kv[opt[0]] = unquote(opt[1])
		} else {
			baseArgs = append(baseArgs, t)
		}
	}

	kv["_args"] = strings.Join(baseArgs, " ")
	return kv
}

func unquote(s string) string {
	if n, e := strconv.Unquote(s); e == nil {
		return n
	}
	return s
}
