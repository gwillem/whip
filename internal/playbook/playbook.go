package playbook

import (
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/charmbracelet/log"
	"github.com/gwillem/whip/internal/model"
	"github.com/gwillem/whip/internal/parser"
	"github.com/gwillem/whip/internal/runners"
	"github.com/mitchellh/mapstructure"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

const (
	defaultAssetPath = "files"
)

var StringToSliceSep = regexp.MustCompile(`,\s*`)

func Load(path string) (*model.Playbook, error) {
	rawData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var anyMap interface{}
	if e := yaml.Unmarshal(rawData, &anyMap); e != nil {
		return nil, e
	}

	pb, err := yamlToPlaybook(anyMap)
	if err != nil {
		return nil, err
	}

	for _, pb := range *pb {
		for _, t := range pb.Hosts {
			log.Debug("Found playbook target:", t)
		}
	}

	expandPlaybookLoops(pb)
	return pb, nil
}

// parses given playbook for asset references, returning a list of unique assets
// func LoadAssets(pb *model.Playbook) (*model.Asset, error) {
// 	if _, err := os.Stat(defaultAssetPath); os.IsNotExist(err) {
// 		return &model.Asset{}, nil
// 	}
// 	return DirToAsset(defaultAssetPath)
// }

func yamlToPlaybook(y any) (*model.Playbook, error) {
	pb := model.Playbook{}
	md := mapstructure.Metadata{}

	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &pb,
		Metadata:         &md,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			parseTasksFunc(),
			parseStringToSlice(),
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

	log.Debug("Unused fields from playbook source:", md.Unused)
	return &pb, nil
}

func parseTasksFunc() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(model.Task{}) {
			return data, nil
		}
		if f != reflect.TypeOf(map[string]any{}) {
			return nil, fmt.Errorf("expected map[string]any{}, got %v", f)
		}
		for k, v := range data.(map[string]any) {
			if slices.Contains(runners.All(), k) {
				delete(data.(map[string]any), k)

				if data.(map[string]any)["runner"] != nil {
					return nil, fmt.Errorf("single task cannot have multiple runners (%s and %s)", data.(map[string]any)["runner"], k)
				}

				data.(map[string]any)["runner"] = k
				switch v.(type) {
				case string:
					data.(map[string]any)["args"] = parser.ParseArgString(v.(string))
				case map[string]any:
					data.(map[string]any)["args"] = v
				default:
					return nil, fmt.Errorf("unexpected type for task arg: %v", v)
				}
				continue
			}
		}
		return data, nil
	}
}

func parseStringToSlice() mapstructure.DecodeHookFunc {
	return func(f, t reflect.Kind, data interface{}) (interface{}, error) {
		if f != reflect.String || t != reflect.Slice {
			return data, nil
		}
		return StringToSliceSep.Split(data.(string), -1), nil
	}
}

// expandPlaybookLoops takes a playbook and expands any tasks that have a Loop,
// replacing them with multiple tasks, each loop item copied into task.Vars
func expandPlaybookLoops(pb *model.Playbook) {
	for playidx := range *pb {
		play := &(*pb)[playidx]
		// fmt.Println("len tasks BEFORE", len(play.Tasks))
		for i := len(play.Tasks) - 1; i >= 0; i-- { // reverse range, because we are expanding the slice in place
			if loops := play.Tasks[i].Loop; loops != nil {
				newTasks := []model.Task{}
				for _, l := range loops {
					newTask := play.Tasks[i].Clone()
					newTask.Vars["item"] = l
					newTask.Loop = nil
					newTasks = append(newTasks, newTask)
				}
				// remove this task from the playbook
				// and insert len(Loop) new tasks in its place
				play.Tasks = slices.Replace(play.Tasks, i, i+1, newTasks...)
				// fmt.Println("expanded", len(newTasks), "tasks..")
			}
		}
		// fmt.Println("len tasks AFTER", len(play.Tasks))
	}
}
