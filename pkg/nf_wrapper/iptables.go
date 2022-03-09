package nf_wrapper

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	Iptables = "iptables"

	//tables
	IptablesTableNat    = "nat"
	IptablesTableRaw    = "raw"
	IptablesTableFilter = "filter"

	//chains
	IptablesChainPrerouting       = "PREROUTING"
	IptablesChainPostrouting      = "POSTROUTING"
	IptablesChainOutput           = "OUTPUT"
	IptablesChainCustomPrerouting = "CustomPrerouting"
	IptablesChainCustomOutput     = "CustomOutput"

	//operations on chain
	IptablesChainAdd   = "A"
	IptablesChainCheck = "C"
	IptablesChainDel   = "D"
	IptablesChainI     = "I"
	IptablesChainFlush = "F"
	IptablesChainNew   = "N"
	IptablesChainX     = "X"
	IptablesChainZ     = "Z"
	//target
	IptablesTargetAccept   = "ACCEPT"
	IptablesTargetReturn   = "RETURN"
	IptablesTargetDrop     = "DROP"
	IptablesTargetRedirect = "REDIRECT" //归属iptables-extensions(man 8)
	IptablesTargetDNAT     = "DNAT"

	//protocol
	IptablesIPv4Icmp = "icmp"
	IptablesIPv4Tcp  = "tcp"
	IptablesIPv4Udp  = "udp"
)

type OptionIptables func(args []string) ([]string, error)

func OptionIptablesTable(table string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "-t", table), nil
	}
}

func OptionIptablesChainOperate(operation string) OptionIptables {
	return func(args []string) ([]string, error) {
		var op string
		switch operation {
		case IptablesChainAdd:
			op = "-A"
		case IptablesChainDel:
			op = "-D"
		case IptablesChainFlush:
			op = "-F"
		case IptablesChainNew:
			op = "-N"
		case IptablesChainX:
			op = "-X"
		case IptablesChainI:
			op = "-I"
		default:
			return nil, errors.New("unsupported operation")
		}
		return append(args, op), nil
	}
}

func OptionIptablesWait() OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "--wait"), nil
	}
}

func OptionIptablesChain(chain string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, chain), nil
	}
}

func OptionIptablesJump(target string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "-j", target), nil
	}
}

func OptionIptableGo(target string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "-g", target), nil
	}
}

//例如
func OptionIptablesJumpSubOptions(options ...string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, options...), nil
	}
}

//protocol
func OptionIptablesIPv4Proto(proto string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "-p", proto), nil
	}
}

func OptionIptablesInIf(nic string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "-i", nic), nil
	}
}

func OptionIptablesIPv4SrcIp(ip string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "-s", ip), nil
	}
}

func OptionIptablesIPv4DstIp(ip string) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "-d", ip), nil
	}
}

func OptionIptablesIPv4SrcPort(port uint32) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "--sport", strconv.Itoa(int(port))), nil
	}
}

func OptionIptablesIPv4DstPort(port uint32) OptionIptables {
	return func(args []string) ([]string, error) {
		return append(args, "--dport", strconv.Itoa(int(port))), nil
	}
}

func IptablesRun(options ...OptionIptables) ([]byte, []byte, error) {
	var err error
	args := []string{}
	for _, option := range options {
		args, err = option(args)
		if err != nil {
			return nil, nil, err
		}
	}
	return Cmd(Iptables, args...)
}

func Exist(table string, chain string, options ...OptionIptables) bool {
	var err error
	args := make([]string, 0)
	for _, option := range options {
		args, err = option(args)
		if err != nil {
			return false
		}
	}
	ruleString := fmt.Sprintf("%s %s\n", chain, strings.Join(args, " "))
	out, _, _ := Cmd(Iptables, "-t", table, "-S", chain)

	return strings.Contains(string(out), ruleString)
}

// ExistChain checks if a chain exists
func ExistChain(table string, chain string) bool {
	_, _, err := Cmd(Iptables, "-t", table, "-nL", chain)
	return err == nil
}
