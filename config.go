package tfjson

import (
	"encoding/json"
	"errors"
)

// Config represents the complete configuration source
type Config struct {
	// A map of all provider instances across all modules in the
	// configuration, indexed in the format NAME.ALIAS. Default
	// providers will not have an alias.
	ProviderConfigs map[string]ProviderConfig `json:"provider_config,omitempty"`

	// The root module in the configuration. Any child modules descend
	// off of here.
	RootModule *ConfigModule `json:"root_module,omitempty"`
}

// Validate checks to ensure that the config is present.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	return nil
}

// ProviderConfig describes a provider configuration instance.
type ProviderConfig struct {
	// The name of the provider, ie: "aws".
	Name string `json:"name,omitempty"`

	// The alias of the provider, ie: "us-east-1".
	Alias string `json:"alias,omitempty"`

	// The address of the module the provider is declared in.
	ModuleAddress string `json:"module_address,omitempty"`

	// Any non-special configuration values in the provider, indexed by
	// key.
	Expressions map[string]Expression `json:"expressions,omitempty"`
}

// ConfigModule describes a module in Terraform configuration.
type ConfigModule struct {
	// The outputs defined in the module.
	Outputs map[string]ConfigOutput `json:"outputs,omitempty"`

	// The resources defined in the module.
	Resources []ConfigResource `json:"resources,omitempty"`

	// Any "module" stanzas within the specific module.
	ModuleCalls map[string]ModuleCall `json:"module_calls,omitempty"`

	// The variables defined in the module.
	Variables map[string]ConfigVariable `json:"variables,omitempty"`
}

// ConfigOutput defines an output as defined in configuration.
type ConfigOutput struct {
	// Indicates whether or not the output was marked as sensitive.
	Sensitive bool `json:"sensitive,omitempty"`

	// The defined value of the output.
	Expression *Expression `json:"expression,omitempty"`
}

// ConfigResource is the configuration representation of a resource.
type ConfigResource struct {
	// The address of the resource relative to the module that it is
	// in.
	Address string `json:"address,omitempty"`

	// The resource mode.
	Mode ResourceMode `json:"mode,omitempty"`

	// The type of resource, ie: "null_resource" in
	// "null_resource.foo".
	Type string `json:"type,omitempty"`

	// The name of the resource, ie: "foo" in "null_resource.foo".
	Name string `json:"name,omitempty"`

	// The provider address used for this resource. This address should
	// be able to be referenced in the ProviderConfig key in the
	// root-level Config structure.
	ProviderConfigKey string `json:"provider_config_key,omitempty"`

	// The list of provisioner defined for this configuration. This
	// will be nil if no providers are defined.
	Provisioners []ConfigProvisioner `json:"provisioners,omitempty"`

	// Any non-special configuration values in the resource, indexed by
	// key.
	Expressions map[string]Expression

	// The resource's configuration schema version. With access to the
	// specific Terraform provider for this resource, this can be used
	// to determine the correct schema for the configuration data
	// supplied in Expressions.
	SchemaVersion uint64 `json:"schema_version"`

	// The expression data for the "count" value in the resource.
	CountExpression *Expression `json:"count_expression,omitempty"`

	// The expression data for the "for_each" value in the resource.
	ForEachExpression *Expression `json:"for_each_expression,omitempty"`

	// RawExpressions represents the raw JSON expression data, which
	// requires special handling to deal with nested blocks. This field
	// is internal and used only during parsing.
	RawExpressions map[string]json.RawMessage `json:"expressions,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for ConfigResource.
func (r *ConfigResource) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}

	r.Expressions = make(map[string]Expression)
	for k, raw := range r.RawExpressions {
		expr, err := unmarshalExpression(raw)
		if err != nil {
			return err
		}

		r.Expressions[k] = expr
	}

	r.RawExpressions = nil
	return nil
}

func unmarshalExpression(raw json.RawMessage) (Expression, error) {
	// Check to see if this is an array first. If it is, this is more
	// than likely a list of nested blocks.
	var rawNested []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawNested); err == nil {
		return unmarshalExpressionBlocks(rawNested)
	}

	var result Expression
	if err := json.Unmarshal(raw, &result); err != nil {
		return Expression{}, err
	}

	return result, nil
}

func unmarshalExpressionBlocks(raw []map[string]json.RawMessage) (Expression, error) {
	var result Expression

	for _, rawBlock := range raw {
		block := make(map[string]Expression)
		for k, rawExpr := range rawBlock {
			expr, err := unmarshalExpression(rawExpr)
			if err != nil {
				return Expression{}, err
			}

			block[k] = expr
		}

		result.NestedBlocks = append(result.NestedBlocks, block)
	}

	return result, nil
}

// ConfigVariable defines a variable as defined in configuration.
type ConfigVariable struct {
	// The defined default value of the variable.
	Default interface{} `json:"default,omitempty"`
	// The defined text description of the variable.
	Description string `json:"description,omitempty"`
}

// ConfigProvisioner describes a provisioner declared in a resource
// configuration.
type ConfigProvisioner struct {
	// The type of the provisioner, ie: "local-exec".
	Type string `json:"type,omitempty"`

	// Any non-special configuration values in the provisioner, indexed by
	// key.
	Expressions map[string]Expression `json:"expressions,omitempty"`
}

// ModuleCall describes a declared "module" within a configuration.
// It also contains the data for the module itself.
type ModuleCall struct {
	// The resolved contents of the "source" field.
	ResolvedSource string `json:"resolved_source,omitempty"`

	// Any non-special configuration values in the module, indexed by
	// key.
	Expressions map[string]Expression `json:"expressions,omitempty"`

	// The expression data for the "count" value in the module.
	CountExpression *Expression `json:"count_expression,omitempty"`

	// The expression data for the "for_each" value in the module.
	ForEachExpression *Expression `json:"for_each_expression,omitempty"`

	// The configuration data for the module itself.
	Module *ConfigModule `json:"module,omitempty"`
}

// Expression describes the format for an individual key in a
// Terraform configuration.
//
// This is usually indexed by key when referenced, ie:
// map[string]Expression, but exceptions exist, such as "count"
// expressions.
type Expression struct {
	// If the *entire* expression is a constant-defined value, this
	// will contain the Go representation of the expression's data.
	ConstantValue interface{} `json:"constant_value,omitempty"`

	// If any part of the expression contained values that were not
	// able to be resolved at parse-time, this will contain a list of
	// the referenced identifiers that caused the value to be unknown.
	References []string `json:"references,omitempty"`

	// A list of complex objects that were nested in this expression.
	// If this value is a nested block in configuration, sometimes
	// referred to as a "sub-resource", this field will contain those
	// values, and ConstantValue and References will be blank.
	NestedBlocks []map[string]Expression
}
