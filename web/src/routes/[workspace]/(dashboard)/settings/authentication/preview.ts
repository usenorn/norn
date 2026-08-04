import type { SsoOutcome, SsoProviderConfiguration } from "$lib/workspace/sso";

export type AuthenticationPreview = {
	configuration?: SsoProviderConfiguration;
	outcome?: SsoOutcome;
	discovering?: boolean;
};

export const authenticationPreviewStates: Record<string, AuthenticationPreview> = import.meta.env
	.DEV
	? {
			loading: { configuration: { kind: "loading" } },
			unconfigured: { configuration: { kind: "unconfigured" } },
			discovering: { configuration: { kind: "unconfigured" }, discovering: true },
			forbidden: { configuration: { kind: "forbidden" } },
			unavailable: { configuration: { kind: "unavailable" } },
			unverified: {
				configuration: {
					kind: "oidc",
					connection: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						endpoints: {
							issuer: "https://login.northwind.co/realms/staff",
							authorizationEndpoint:
								"https://login.northwind.co/realms/staff/protocol/openid-connect/auth",
							tokenEndpoint:
								"https://login.northwind.co/realms/staff/protocol/openid-connect/token",
							jwksUri: "https://login.northwind.co/realms/staff/protocol/openid-connect/certs",
							userinfoEndpoint:
								"https://login.northwind.co/realms/staff/protocol/openid-connect/userinfo",
						},
						discovered: true,
						clientId: "norn-dashboard",
						secretSet: true,
						scopes: ["openid", "email", "profile"],
						provisioning: false,
						redirectUri: "https://norn.northwind.co/v1/sso/oidc/callback",
						updatedAt: "2026-08-03T10:15:00Z",
					},
				},
			},
			verified: {
				configuration: {
					kind: "oidc",
					connection: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						endpoints: {
							issuer: "https://login.northwind.co/realms/staff",
							authorizationEndpoint:
								"https://login.northwind.co/realms/staff/protocol/openid-connect/auth",
							tokenEndpoint:
								"https://login.northwind.co/realms/staff/protocol/openid-connect/token",
							jwksUri: "https://login.northwind.co/realms/staff/protocol/openid-connect/certs",
						},
						discovered: true,
						clientId: "norn-dashboard",
						secretSet: true,
						scopes: ["openid", "email", "profile", "groups"],
						groupsClaim: "groups",
						provisioning: true,
						redirectUri: "https://norn.northwind.co/v1/sso/oidc/callback",
						verifiedAt: "2026-08-03T10:22:00Z",
						updatedAt: "2026-08-03T10:15:00Z",
					},
				},
				outcome: { kind: "verified" },
			},
			manual: {
				configuration: {
					kind: "oidc",
					connection: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						endpoints: {
							issuer: "https://sso.northwind.co",
							authorizationEndpoint: "https://sso.northwind.co/oauth2/authorize",
							tokenEndpoint: "https://sso.northwind.co/oauth2/token",
							jwksUri: "https://sso.northwind.co/oauth2/keys",
						},
						discovered: false,
						clientId: "norn-dashboard",
						secretSet: true,
						scopes: ["openid", "email", "profile"],
						provisioning: false,
						redirectUri: "https://norn.northwind.co/v1/sso/oidc/callback",
						updatedAt: "2026-08-03T09:00:00Z",
					},
				},
			},
			saved: { configuration: { kind: "unconfigured" }, outcome: { kind: "saved" } },
			removed: { configuration: { kind: "unconfigured" }, outcome: { kind: "removed" } },
			saml_unverified: {
				configuration: {
					kind: "saml",
					connection: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						descriptor: {
							entityId: "https://login.northwind.co/realms/staff",
							ssoUrl: "https://login.northwind.co/realms/staff/protocol/saml",
							sloUrl: "https://login.northwind.co/realms/staff/protocol/saml",
							certificates: ["MIIClzCCAX8CBgGfyvWDgDANBgkqhkiG9w0BAQsFADAP"],
							expiresAt: "2027-08-04T00:00:00Z",
						},
						providerMetadataUrl:
							"https://login.northwind.co/realms/staff/protocol/saml/descriptor",
						spEntityId: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						spCertificate: "MIIDazCCAlOgAwIBAgIUNorthwindServiceProviderCert",
						acsUrl: "https://norn.northwind.co/v1/sso/saml/northwind/acs",
						metadataUrl: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						signInUrl: "https://norn.northwind.co/sso?workspace=northwind",
						allowIdpInitiated: false,
						mapping: {},
						provisioning: false,
						certificateExpiresAt: "2027-08-04T00:00:00Z",
						certificateDaysLeft: 365,
						updatedAt: "2026-08-04T10:15:00Z",
					},
				},
			},
			saml_verified: {
				configuration: {
					kind: "saml",
					connection: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						descriptor: {
							entityId: "https://login.northwind.co/realms/staff",
							ssoUrl: "https://login.northwind.co/realms/staff/protocol/saml",
							certificates: ["MIIClzCCAX8CBgGfyvWDgDANBgkqhkiG9w0BAQsFADAP"],
							expiresAt: "2027-08-04T00:00:00Z",
						},
						spEntityId: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						spCertificate: "MIIDazCCAlOgAwIBAgIUNorthwindServiceProviderCert",
						acsUrl: "https://norn.northwind.co/v1/sso/saml/northwind/acs",
						metadataUrl: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						allowIdpInitiated: true,
						mapping: { email: "urn:oid:0.9.2342.19200300.100.1.3" },
						provisioning: true,
						certificateExpiresAt: "2027-08-04T00:00:00Z",
						certificateDaysLeft: 365,
						verifiedAt: "2026-08-04T10:22:00Z",
						updatedAt: "2026-08-04T10:15:00Z",
					},
				},
				outcome: { kind: "verified" },
			},
			saml_certificate_expiring: {
				configuration: {
					kind: "saml",
					connection: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						descriptor: {
							entityId: "https://login.northwind.co/realms/staff",
							ssoUrl: "https://login.northwind.co/realms/staff/protocol/saml",
							certificates: ["MIIClzCCAX8CBgGfyvWDgDANBgkqhkiG9w0BAQsFADAP"],
							expiresAt: "2026-08-11T00:00:00Z",
						},
						spEntityId: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						spCertificate: "MIIDazCCAlOgAwIBAgIUNorthwindServiceProviderCert",
						acsUrl: "https://norn.northwind.co/v1/sso/saml/northwind/acs",
						metadataUrl: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						allowIdpInitiated: false,
						mapping: {},
						provisioning: true,
						certificateExpiresAt: "2026-08-11T00:00:00Z",
						certificateDaysLeft: 7,
						verifiedAt: "2026-08-04T10:22:00Z",
						updatedAt: "2026-08-04T10:15:00Z",
					},
				},
			},
			saml_certificate_expired: {
				configuration: {
					kind: "saml",
					connection: {
						workspaceId: "00000000-0000-4000-8000-000000000000",
						descriptor: {
							entityId: "https://login.northwind.co/realms/staff",
							ssoUrl: "https://login.northwind.co/realms/staff/protocol/saml",
							certificates: ["MIIClzCCAX8CBgGfyvWDgDANBgkqhkiG9w0BAQsFADAP"],
							expiresAt: "2026-07-28T00:00:00Z",
						},
						spEntityId: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						spCertificate: "MIIDazCCAlOgAwIBAgIUNorthwindServiceProviderCert",
						acsUrl: "https://norn.northwind.co/v1/sso/saml/northwind/acs",
						metadataUrl: "https://norn.northwind.co/v1/sso/saml/northwind/metadata",
						allowIdpInitiated: false,
						mapping: {},
						provisioning: true,
						certificateExpiresAt: "2026-07-28T00:00:00Z",
						certificateDaysLeft: -7,
						verifiedAt: "2026-07-01T10:22:00Z",
						updatedAt: "2026-07-01T10:15:00Z",
					},
				},
			},
			failed_metadata: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "metadata", message: "Something went wrong at the metadata stage." },
				},
			},
			failed_certificate: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "certificate", message: "Something went wrong at the certificate stage." },
				},
			},
			failed_request: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "request", message: "Something went wrong at the request stage." },
				},
			},
			failed_response: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "response", message: "Something went wrong at the response stage." },
				},
			},
			failed_signature: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "signature", message: "Something went wrong at the signature stage." },
				},
			},
			failed_conditions: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "conditions", message: "Something went wrong at the conditions stage." },
				},
			},
			failed_replay: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "replay", message: "Something went wrong at the replay stage." },
				},
			},
			failed_attributes: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: { kind: "stage", stage: "attributes", message: "Something went wrong at the attributes stage." },
				},
			},
			failed_discovery: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "discovery",
						message: "Norn could not read the discovery document at that issuer.",
						providerMessage: "404 Not Found",
					},
				},
			},
			failed_endpoints: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "endpoints",
						message: "The token endpoint is not a usable https address.",
					},
				},
			},
			failed_jwks: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "jwks",
						message: "The signing keys could not be fetched.",
						providerMessage: "dial tcp: i/o timeout",
					},
				},
			},
			failed_authorization: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "authorization",
						message: "Your provider refused the sign-in.",
						providerMessage: "access_denied: user is not assigned to the client application",
					},
				},
			},
			failed_token_exchange: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "token_exchange",
						message: "The provider refused to exchange the sign-in code for a token.",
						providerMessage: "invalid_client: Invalid client or Invalid client credentials",
					},
				},
			},
			failed_id_token: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "id_token",
						message: "The ID token from the provider could not be verified.",
						providerMessage: "oidc: expected audience \"norn-dashboard\" got [\"other-app\"]",
					},
				},
			},
			failed_claims: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "claims",
						message:
							"The provider did not return an email address. Add the email scope to the client.",
					},
				},
			},
			failed_matching: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "matching",
						message:
							"rae@northwind.co signed in with your provider but is not a member of this workspace.",
					},
				},
			},
			failed_provisioning: {
				configuration: { kind: "unconfigured" },
				outcome: {
					kind: "failed",
					failure: {
						kind: "stage",
						stage: "provisioning",
						message:
							"There is no Norn account for rae@northwind.co, and just-in-time provisioning is turned off.",
					},
				},
			},
		}
	: {};
