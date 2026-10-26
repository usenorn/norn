package entity

import "time"

const SignInDiscoveryFloor = 400 * time.Millisecond

type SignInWorkspace struct {
	Slug     string
	Name     string
	Protocol SSOProtocol
	Host     string
}

type SignInOption struct {
	Method    SessionAuthMethod
	Workspace *SignInWorkspace
}

type SignInOptions struct {
	Options   []SignInOption
	Preferred SessionAuthMethod
}

type WorkspaceSignInPolicy struct {
	Workspace   SignInWorkspace
	Enforcement AuthEnforcement
	SSOReady    bool
}

type WorkspaceSignIn struct {
	Slug string
	Name string
	SignInOptions
}

func DeriveSignInOptions(passwordAuth bool, policies []WorkspaceSignInPolicy) SignInOptions {
	options := make([]SignInOption, 0, len(policies)+1)
	demandsSSO := len(policies) > 0

	for _, policy := range policies {
		if policy.SSOReady {
			workspace := policy.Workspace
			options = append(options, SignInOption{Method: SessionAuthMethodSSO, Workspace: &workspace})
		}

		if policy.Enforcement.Permits(SessionAuthMethodPassword) {
			demandsSSO = false
		}
	}

	if passwordAuth && (!demandsSSO || len(options) == 0) {
		options = append(options, SignInOption{Method: SessionAuthMethodPassword})
	}

	derived := SignInOptions{Options: options}

	if len(options) > 0 {
		derived.Preferred = options[0].Method
	}

	return derived
}

func (o SignInOptions) Offers(method SessionAuthMethod) bool {
	for _, option := range o.Options {
		if option.Method == method {
			return true
		}
	}

	return false
}
