package cluster

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/flyzstu/RedisSleuth/internal/model"
)

func ParseNodes(raw string) ([]model.Node, error) {
	var nodes []model.Node
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 8 {
			return nil, fmt.Errorf("无效 CLUSTER NODES 行: %q", line)
		}
		flags := strings.Split(f[2], ",")
		role := "replica"
		if contains(flags, "master") {
			role = "master"
		}
		addr := strings.SplitN(f[1], "@", 2)[0]
		node := model.Node{
			ID: f[0], Addr: addr, Role: role, LinkState: f[7], Flags: flags,
		}
		if f[3] != "-" {
			node.MasterID = f[3]
		}
		for _, value := range f[8:] {
			if strings.HasPrefix(value, "[") {
				continue
			}
			if _, err := strconv.Atoi(strings.SplitN(value, "-", 2)[0]); err == nil {
				node.Slots = append(node.Slots, value)
			}
		}
		nodes = append(nodes, node)
	}
	return nodes, scanner.Err()
}

func ParseClusterInfo(raw string) map[string]string { return ParseInfo(raw) }

func ParseInfo(raw string) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if ok {
			result[k] = strings.TrimSpace(v)
		}
	}
	return result
}

func ParseClientList(raw string) ([]model.Client, error) {
	var clients []model.Client
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		values := make(map[string]string)
		for _, field := range strings.Fields(scanner.Text()) {
			k, v, ok := strings.Cut(field, "=")
			if ok {
				values[k] = v
			}
		}
		host, port := splitClientAddr(values["addr"])
		age, _ := strconv.ParseInt(values["age"], 10, 64)
		idle, _ := strconv.ParseInt(values["idle"], 10, 64)
		db, _ := strconv.Atoi(values["db"])
		qbuf, _ := strconv.ParseInt(values["qbuf"], 10, 64)
		obl, _ := strconv.ParseInt(values["obl"], 10, 64)
		omem, _ := strconv.ParseInt(values["omem"], 10, 64)
		clients = append(clients, model.Client{
			IP: host, Port: port, Name: values["name"], Age: age, Idle: idle,
			Flags: values["flags"], DB: db, Command: values["cmd"],
			InputBuf: qbuf, OutputBuf: omem + obl,
		})
	}
	return clients, scanner.Err()
}

func splitClientAddr(addr string) (string, string) {
	if host, port, err := net.SplitHostPort(addr); err == nil {
		return host, port
	}
	i := strings.LastIndex(addr, ":")
	if i > 0 {
		return addr[:i], addr[i+1:]
	}
	return addr, ""
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
