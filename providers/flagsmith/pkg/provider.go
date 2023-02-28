package flagsmith

import (
	"context"
	"fmt"
	"strconv"
	flagsmithClient "github.com/Flagsmith/flagsmith-go-client/v2"
	of "github.com/open-feature/go-sdk/pkg/openfeature"
)


type Provider struct {
	client *flagsmithClient.Client
	usingBooleanConfigValue bool
}

type ProviderOption func(*Provider)

func NewProvider(client *flagsmithClient.Client, opts ...ProviderOption) *Provider {
	provider := &Provider{
		client: client,
	}
	for _, opt := range opts {
		opt(provider)
	}
	return provider

}

// flagsmith provider does not have any hooks, returns empty slice
func (p *Provider) Hooks() []of.Hook {
	return []of.Hook{}
}

// Metadata returns value of Metadata (name of current service, exposed to openfeature sdk)
func (provider *Provider) Metadata() of.Metadata {
	return of.Metadata{
		Name: "Flagsmith",
	}
}

const TraitsKey = "traits"

func (p *Provider) resolveFlag(ctx context.Context, flag string, defaultValue interface{}, evalCtx of.FlattenedContext) of.InterfaceResolutionDetail {
	var flags flagsmithClient.Flags
	var err error

	reason := of.StaticReason

	_, targetKeyFound := evalCtx[of.TargetingKey]

	if targetKeyFound {
		reason = of.TargetingMatchReason
		targetKey, ok := evalCtx[of.TargetingKey].(string)
		if !ok {
			e := of.NewInvalidContextResolutionError("flagsmith: targeting key is not a string")
			return of.InterfaceResolutionDetail{
				Value: defaultValue,
				ProviderResolutionDetail: of.ProviderResolutionDetail{
					ResolutionError: e,
					Reason:          of.ErrorReason,
				},
			}

		}
		var traits []*flagsmithClient.Trait
		userTraits, ok := evalCtx[TraitsKey]
		if ok {
			traits, ok = userTraits.([]*flagsmithClient.Trait)
			if !ok {
				e := of.NewInvalidContextResolutionError(fmt.Sprintf("flagsmith: invalid traits: expected type []*flagsmithClient.Trait, got %T", userTraits))
				return of.InterfaceResolutionDetail{
					Value: defaultValue,
					ProviderResolutionDetail: of.ProviderResolutionDetail{
						ResolutionError: e,
						Reason:          of.ErrorReason,
					},
				}
			}
		}
		flags, err = p.client.GetIdentityFlags(targetKey, traits)
		if err != nil {
			var e of.ResolutionError
			e = of.NewGeneralResolutionError(err.Error())
			return of.InterfaceResolutionDetail{
				Value: defaultValue,
				ProviderResolutionDetail: of.ProviderResolutionDetail{
					ResolutionError: e,
					Reason:          of.ErrorReason,
				},
			}
		}

	} else {
		flags, err = p.client.GetEnvironmentFlags()
		if err != nil {
			var e of.ResolutionError
			e = of.NewGeneralResolutionError(err.Error())

			return of.InterfaceResolutionDetail{
				Value: defaultValue,
				ProviderResolutionDetail: of.ProviderResolutionDetail{
					ResolutionError: e,
					Reason:          of.ErrorReason,
				},
			}
		}
	}

	flagObj, err := flags.GetFlag(flag)
	if err != nil {
		e := of.NewFlagNotFoundResolutionError(err.Error())
		return of.InterfaceResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: e,
				Reason:          of.ErrorReason,
			},
		}
	}
	if !flagObj.Enabled {
		return of.InterfaceResolutionDetail{
			Value:  flagObj.Value,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				Reason:          of.DisabledReason,
			},
		}
	}
	return of.InterfaceResolutionDetail{
		Value: flagObj.Value,
		ProviderResolutionDetail: of.ProviderResolutionDetail{
			Reason: reason,
		},
	}

}
func (p *Provider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, evalCtx of.FlattenedContext) of.BoolResolutionDetail {
	res := p.resolveFlag(ctx, flag, defaultValue, evalCtx)

	if res.Error()!= nil {
		return of.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: res.ProviderResolutionDetail,
		}
	}


	if p.usingBooleanConfigValue {
		value := !(res.ProviderResolutionDetail.Reason == of.DisabledReason)
		return of.BoolResolutionDetail{
			Value: value,
			ProviderResolutionDetail: res.ProviderResolutionDetail,
		}
	}

	value, ok := res.Value.(bool)
	if !ok {
		return of.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: of.NewTypeMismatchResolutionError(fmt.Sprintf("flagsmith: Value %v is not a valid boolean", res.Value)),
				Reason:          of.ErrorReason,
			},
		}
	}
	return of.BoolResolutionDetail{
		Value:                    value,
		ProviderResolutionDetail: res.ProviderResolutionDetail,
	}
}

func (p *Provider) StringEvaluation(ctx context.Context, flag string, defaultValue string, evalCtx of.FlattenedContext) of.StringResolutionDetail {
	res := p.resolveFlag(ctx, flag, defaultValue, evalCtx)

	if res.Error()!= nil {
		return of.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: res.ProviderResolutionDetail,
		}
	}


	value, ok := res.Value.(string)
	if !ok {
		return of.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: of.NewTypeMismatchResolutionError(fmt.Sprintf("flagsmith: Value %v is not a valid string", res.Value)),
				Reason:          of.ErrorReason,
			},
		}
	}

	return of.StringResolutionDetail{
		Value: value,
		ProviderResolutionDetail: res.ProviderResolutionDetail,
	}
}

func (p *Provider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, evalCtx of.FlattenedContext) of.FloatResolutionDetail {
	res := p.resolveFlag(ctx, flag, defaultValue, evalCtx)

	if res.Error()!= nil {
		return of.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: res.ProviderResolutionDetail,
		}
	}

	misMatachResolutionErr := of.NewTypeMismatchResolutionError(fmt.Sprintf("flagsmith: Value %v is not a valid float", res.Value))

	// We store floats as string
	stringValue, ok := res.Value.(string)
	if !ok {
		return of.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: misMatachResolutionErr,
				Reason:          of.ErrorReason,
			},
		}
	}
	// Convert sting back to float64
	value, err := strconv.ParseFloat(stringValue, 64)
	if err != nil {
		return of.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: misMatachResolutionErr,
				Reason:          of.ErrorReason,
			},
		}
	}

	return of.FloatResolutionDetail{
		Value: value,
		ProviderResolutionDetail: res.ProviderResolutionDetail,
	}

}

func (p *Provider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, evalCtx of.FlattenedContext) of.IntResolutionDetail {
	res := p.resolveFlag(ctx, flag, defaultValue, evalCtx)
	if res.Error()!= nil {
		return of.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: res.ProviderResolutionDetail,
		}
	}

	// Because `encoding/json` uses float64 for JSON numbers
	//ref: https://pkg.go.dev/encoding/json#Unmarshal
	value, ok := res.Value.(float64)
	if !ok {
		return of.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: of.ProviderResolutionDetail{
				ResolutionError: of.NewTypeMismatchResolutionError(fmt.Sprintf("flagsmith: Value %v is not a valid int", res.Value)),
				Reason:          of.ErrorReason,
			},
		}
	}

	// Convert the float64 back to integer
	int64Value := int64(value)
	return of.IntResolutionDetail{
		Value: int64Value,
		ProviderResolutionDetail: res.ProviderResolutionDetail,
	}
}

func (p *Provider) ObjectEvaluation(ctx context.Context, flag string, defaultValue interface{}, evalCtx of.FlattenedContext) of.InterfaceResolutionDetail {
	return p.resolveFlag(ctx, flag, defaultValue, evalCtx)
}

// WithBooleanConfigValue configures the provider to use the result of isFeatureEnabled as the boolean value of the flag
// i.e: if the flag is enabled, the value will be true, otherwise it will be false
func WithUsingBooleanConfigValue() ProviderOption {
	return func(p *Provider) {
		p.usingBooleanConfigValue = true
	}
}


