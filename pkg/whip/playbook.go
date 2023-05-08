package whip

/*

Need to do custom YAML parsing, because Ansible syntax uses dynamic dict
keys as plugin names

*/

import (
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
		play.Hosts = rawPlay.Hosts
		for _, rawTask := range rawPlay.RawTasks {
			task := Task{
				Args: map[string]string{},
			}
			for k, v := range rawTask {

				if k == "name" {
					task.Name = v.(string)
					continue
				}

				task.Type = k
				if val, ok := v.(string); ok {
					task.Args = parseArgString(val)
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

	kv["args"] = strings.Join(baseArgs, " ")
	return kv
}
