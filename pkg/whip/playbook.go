package whip

/*

Need to do custom YAML parsing, because Ansible syntax uses dynamic dict
keys as plugin names

*/

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadPlaybook(path string) Playbook {
	yamlData, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var rawPb = RawPlaybook{}
	var pb = Playbook{}

	if e := yaml.Unmarshal(yamlData, &rawPb); e != nil {
		panic(e)
	}

	for _, rawPlay := range rawPb {
		play := Play{}

		play.Hosts = parseHosts(rawPlay.Hosts)
		// play.Targets = rawPlay.Targets
		for _, rawTask := range rawPlay.RawTasks {
			task := Task{Args: TaskArgs{}}
			for k, v := range rawTask {

				switch k {
				case "name":
					task.Name = v.(string)
					continue
				case "with_items":
					task.Items = v.([]any)
					continue
				}

				task.Type = k
				if val, ok := v.(string); ok {
					task.Args = TaskArgs{}
					for k, v := range parseArgString(val) {
						task.Args[k] = v
					}
				}

				if val, ok := v.(AnyMap); ok {
					for k, v := range val {
						task.Args[k] = v.(string)
					}
				}
			}
			play.Tasks = append(play.Tasks, task)
		}
		pb = append(pb, play)
	}
	return pb
}

func parseHosts(hosts any) []Host {
	ret := []Host{}
	switch t := hosts.(type) {
	case string:
		for _, t := range strings.Split(hosts.(string), ",") {
			ret = append(ret, Host(strings.TrimSpace(t)))
		}
		return ret
	case []string:
		for _, t := range hosts.([]string) {
			fmt.Println("found string", t)
			ret = append(ret, Host(t))
		}
		return ret
	default:
		panic(fmt.Sprintf("unknown hosts field %s", t))
	}
}

func parseArgString(arg string) map[string]string {
	kv := map[string]string{}

	baseArgs := []string{}
	for _, t := range strings.Split(arg, " ") {
		if strings.Contains(t, "=") {
			opt := strings.SplitN(t, "=", 2)
			kv[opt[0]] = opt[1]
		} else {
			baseArgs = append(baseArgs, t)
		}
	}

	kv["_args"] = strings.Join(baseArgs, " ")
	return kv
}
