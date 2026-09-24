package mcpregistry

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/toolingclient"
	"github.com/usenorn/norn/internal/repository"
)

const (
	serversPath         = "/v0/servers"
	latestVersion       = "latest"
	activeStatus        = "active"
	officialMeta        = "io.modelcontextprotocol.registry/official"
	stdioTransport      = "stdio"
	streamableTransport = "streamable-http"
	sseTransport        = "sse"
	npmRegistry         = "npm"
	pypiRegistry        = "pypi"
	ociRegistry         = "oci"
	positionalArgument  = "positional"
	namedArgument       = "named"
	npxCommand          = "npx"
	npxAssumeYes        = "-y"
	npmScope            = "@"
	uvxCommand          = "uvx"
	dockerCommand       = "docker"
)

type argument struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Value     string `json:"value"`
	ValueHint string `json:"valueHint"`
}

type variable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsRequired  bool   `json:"isRequired"`
	IsSecret    bool   `json:"isSecret"`
	Default     string `json:"default"`
	Value       string `json:"value"`
}

type transport struct {
	Type string `json:"type"`
}

type pkg struct {
	RegistryType         string     `json:"registryType"`
	Identifier           string     `json:"identifier"`
	Version              string     `json:"version"`
	RuntimeHint          string     `json:"runtimeHint"`
	Transport            transport  `json:"transport"`
	RuntimeArguments     []argument `json:"runtimeArguments"`
	PackageArguments     []argument `json:"packageArguments"`
	EnvironmentVariables []variable `json:"environmentVariables"`
}

type remote struct {
	Type    string     `json:"type"`
	URL     string     `json:"url"`
	Headers []variable `json:"headers"`
}

type server struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	WebsiteURL  string `json:"websiteUrl"`
	Repository  struct {
		URL string `json:"url"`
	} `json:"repository"`
	Packages []pkg    `json:"packages"`
	Remotes  []remote `json:"remotes"`
}

type listing struct {
	Servers []struct {
		Server server `json:"server"`
		Meta   map[string]struct {
			Status string `json:"status"`
		} `json:"_meta"`
	} `json:"servers"`
}

type officialRegistry struct {
	client *toolingclient.Client
}

func New(client *toolingclient.Client) repository.MCPRegistry {
	return &officialRegistry{client: client}
}

func (r *officialRegistry) Search(ctx context.Context, query string) ([]entity.AgentMCPRegistryEntry, error) {
	parameters := url.Values{}
	parameters.Set("version", latestVersion)
	parameters.Set("limit", strconv.Itoa(entity.AgentMCPRegistrySearchMax))

	if trimmed := strings.TrimSpace(query); trimmed != "" {
		parameters.Set("search", trimmed)
	}

	var listed listing

	if err := r.client.GetJSON(
		ctx,
		r.client.RegistryURL(serversPath+"?"+parameters.Encode()),
		nil,
		&listed,
	); err != nil {
		if errors.Is(err, toolingclient.ErrNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("%w: %w", entity.ErrAgentMCPRegistryUnreachable, err)
	}

	entries := make([]entity.AgentMCPRegistryEntry, 0, len(listed.Servers))

	for _, listed := range listed.Servers {
		if meta, ok := listed.Meta[officialMeta]; ok && meta.Status != "" && meta.Status != activeStatus {
			continue
		}

		entry := toEntry(listed.Server)
		if len(entry.Templates) == 0 {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func toEntry(listed server) entity.AgentMCPRegistryEntry {
	entry := entity.AgentMCPRegistryEntry{
		Name:        listed.Name,
		Title:       listed.Title,
		Description: listed.Description,
		Version:     listed.Version,
		WebsiteURL:  listed.WebsiteURL,
		Repository:  listed.Repository.URL,
	}

	for _, remote := range listed.Remotes {
		if template, ok := remoteTemplate(remote); ok {
			entry.Templates = append(entry.Templates, template)
		}
	}

	for _, pkg := range listed.Packages {
		if template, ok := packageTemplate(pkg); ok {
			entry.Templates = append(entry.Templates, template)
		}
	}

	return entry
}

func remoteTemplate(listed remote) (entity.AgentMCPServerTemplate, bool) {
	var transport entity.AgentMCPTransport

	switch listed.Type {
	case streamableTransport:
		transport = entity.AgentMCPHTTP
	case sseTransport:
		transport = entity.AgentMCPSSE
	default:
		return entity.AgentMCPServerTemplate{}, false
	}

	if listed.URL == "" {
		return entity.AgentMCPServerTemplate{}, false
	}

	return entity.AgentMCPServerTemplate{
		Transport: transport,
		URL:       listed.URL,
		Headers:   specs(listed.Headers),
	}, true
}

func packageTemplate(listed pkg) (entity.AgentMCPServerTemplate, bool) {
	if listed.Transport.Type != stdioTransport || listed.Identifier == "" {
		return entity.AgentMCPServerTemplate{}, false
	}

	runtime := arguments(listed.RuntimeArguments)
	env := specs(listed.EnvironmentVariables)

	var (
		command string
		args    []string
	)

	switch listed.RegistryType {
	case npmRegistry:
		command = runtimeOr(listed.RuntimeHint, npxCommand)

		if len(runtime) == 0 && command == npxCommand {
			runtime = []string{npxAssumeYes}
		}

		args = append(runtime, versioned(listed.Identifier, "@", listed.Version))
	case pypiRegistry:
		command = runtimeOr(listed.RuntimeHint, uvxCommand)
		args = append(runtime, versioned(listed.Identifier, "==", listed.Version))
	case ociRegistry:
		command = runtimeOr(listed.RuntimeHint, dockerCommand)
		args = []string{"run", "-i", "--rm"}

		for _, variable := range env {
			args = append(args, "-e", variable.Key)
		}

		args = append(args, runtime...)
		args = append(args, versioned(listed.Identifier, ":", listed.Version))
	default:
		return entity.AgentMCPServerTemplate{}, false
	}

	return entity.AgentMCPServerTemplate{
		Transport: entity.AgentMCPStdio,
		Command:   command,
		Args:      append(args, arguments(listed.PackageArguments)...),
		Env:       env,
	}, true
}

func runtimeOr(hint, fallback string) string {
	if hint != "" {
		return hint
	}

	return fallback
}

func versioned(identifier, separator, version string) string {
	unscoped := strings.TrimPrefix(identifier, npmScope)
	if version == "" || version == latestVersion || strings.Contains(unscoped, separator) {
		return identifier
	}

	return identifier + separator + version
}

func arguments(listed []argument) []string {
	var out []string

	for _, argument := range listed {
		value := argument.Value
		if value == "" && argument.ValueHint != "" {
			value = "<" + argument.ValueHint + ">"
		}

		switch argument.Type {
		case positionalArgument:
			if value != "" {
				out = append(out, value)
			}
		case namedArgument:
			if argument.Name == "" {
				continue
			}

			out = append(out, argument.Name)

			if value != "" {
				out = append(out, value)
			}
		}
	}

	return out
}

func specs(listed []variable) []entity.AgentMCPVariableSpec {
	out := make([]entity.AgentMCPVariableSpec, 0, len(listed))

	for _, variable := range listed {
		if variable.Name == "" {
			continue
		}

		fallback := variable.Default
		if fallback == "" {
			fallback = variable.Value
		}

		out = append(out, entity.AgentMCPVariableSpec{
			Key:         variable.Name,
			Description: variable.Description,
			Required:    variable.IsRequired,
			Secret:      variable.IsSecret,
			Default:     fallback,
		})
	}

	return out
}
