// Command openflare connects to an OpenFlare control plane for routine operations.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/Rain-kl/Wavelet/internal/platform/buildinfo"
	"github.com/spf13/cobra"
)

const (
	defaultProfile = "default"
	jsonOutput     = "json"
)

type cliOptions struct {
	profile string
	output  string
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	err := rootCommand().ExecuteContext(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func rootCommand() *cobra.Command {
	options := new(cliOptions)
	root := &cobra.Command{
		Use: "openflare", Short: "Operate an OpenFlare instance",
		Version: buildinfo.Version, SilenceUsage: true, SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if !profileName.MatchString(options.profile) {
				return errors.New("invalid profile name: use 1–64 letters, digits, underscores or hyphens")
			}
			if options.output != jsonOutput {
				return errors.New("--output must be json")
			}
			return nil
		},
	}
	root.PersistentFlags().Var(&operationFlag{value: defaultProfile, target: &options.profile}, "profile", "Use a named connection instead of the default connection")
	options.profile = defaultProfile
	root.PersistentFlags().StringVar(&options.output, "output", jsonOutput, "Output format (json)")
	operations := automationcontract.Operations()
	for _, operation := range operations {
		registerOperation(root, options, operation)
	}
	root.AddCommand(authCommand(options), catalogCommand(options, operations))
	return root
}

func registerOperation(root *cobra.Command, options *cliOptions, operation automationcontract.Operation) {
	segments := strings.Fields(operation.CLI)
	parent := root
	for _, segment := range segments[:len(segments)-1] {
		parent = commandGroup(parent, segment)
	}
	command := &cobra.Command{
		Use: segments[len(segments)-1], Short: operation.Description, Args: cobra.NoArgs,
	}
	for _, param := range operation.Parameters {
		help := param.Description
		if limit, ok := param.Schema["enum"].([]any); ok {
			help += fmt.Sprintf(" Values: %v.", limit)
		}
		if fallback, ok := param.Schema["default"]; ok {
			help += fmt.Sprintf(" Default: %v.", fallback)
		}
		name := flagName(param)
		switch param.Schema["type"] {
		case automationcontract.SchemaArray:
			help += " Pass a JSON array."
		case automationcontract.SchemaObject:
			help += " Pass a JSON object."
		}
		command.Flags().Var(new(operationFlag), name, help)
		if param.Required {
			if err := command.MarkFlagRequired(name); err != nil {
				panic(err)
			}
		}
	}
	var passwordStdin bool
	if operation.ID == "openflare_site_set_auth" {
		command.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the site password from standard input instead of --password")
		command.MarkFlagsMutuallyExclusive("password", "password-stdin")
	}
	command.RunE = func(command *cobra.Command, _ []string) error {
		parameters := make(map[string]string)
		for _, param := range operation.Parameters {
			flag := command.Flags().Lookup(flagName(param))
			if flag.Changed {
				parameters[param.Name] = flag.Value.String()
			}
		}
		if passwordStdin {
			password, err := readSitePassword(command.InOrStdin())
			if err != nil {
				return err
			}
			parameters["password"] = password
		}
		path, err := operation.PathAndQuery(parameters)
		if err != nil {
			return err
		}
		var body json.RawMessage
		if hasBody(operation) {
			body, err = operation.JSONBody(parameters)
			if err != nil {
				return err
			}
		}
		return runOperation(command, options, operation.Method, path, operation.Revision(), body)
	}
	parent.AddCommand(command)
}

func readSitePassword(input io.Reader) (string, error) {
	// Allow for a trailing line ending and reject oversized input without reading it all.
	const maxPasswordBytes = 255
	const maxInput = maxPasswordBytes + len("\r\n") + 1
	raw, err := io.ReadAll(io.LimitReader(input, int64(maxInput)))
	if err != nil {
		return "", fmt.Errorf("read site password: %w", err)
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if len(raw) == maxInput || len(password) > maxPasswordBytes {
		return "", errors.New("password input exceeds 255 bytes")
	}
	return password, nil
}

func hasBody(operation automationcontract.Operation) bool {
	for _, param := range operation.Parameters {
		if param.In == automationcontract.LocationBody {
			return true
		}
	}
	return false
}

type operationFlag struct {
	value  string
	set    bool
	target *string
}

func (flag *operationFlag) Type() string   { return "value" }
func (flag *operationFlag) String() string { return flag.value }
func (flag *operationFlag) Set(value string) error {
	if flag.set {
		return errors.New("argument may only be specified once")
	}
	flag.value, flag.set = value, true
	if flag.target != nil {
		*flag.target = value
	}
	return nil
}

func flagName(param automationcontract.Parameter) string {
	return strings.ReplaceAll(param.Name, "_", "-")
}

func commandGroup(parent *cobra.Command, name string) *cobra.Command {
	for _, command := range parent.Commands() {
		if command.Name() == name {
			return command
		}
	}
	command := &cobra.Command{Use: name}
	parent.AddCommand(command)
	return command
}

func catalogCommand(options *cliOptions, operations []automationcontract.Operation) *cobra.Command {
	catalog := &cobra.Command{Use: "operations", Short: "Discover automation operations"}
	catalog.AddCommand(&cobra.Command{
		Use: "list", Short: "List operations authorized by the connected instance", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runOperation(command, options, "GET", "/operations", "", nil)
		},
	}, &cobra.Command{
		Use: "describe OPERATION_ID", Short: "Describe a local operation contract", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			for _, operation := range operations {
				if operation.ID == args[0] {
					return json.NewEncoder(command.OutOrStdout()).Encode(operation)
				}
			}
			return fmt.Errorf("unknown operation %q", args[0])
		},
	})
	return catalog
}

func runOperation(command *cobra.Command, options *cliOptions, method, path, revision string, body json.RawMessage) error {
	ctx := command.Context()
	connection, err := activeConnection(ctx, options.profile)
	if err != nil {
		return err
	}
	result, err := connection.call(ctx, method, path, revision, body)
	if err != nil {
		return err
	}
	return json.NewEncoder(command.OutOrStdout()).Encode(result)
}
