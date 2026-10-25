import type { AiProviderOutcome, AiProviderView } from "$lib/ai-provider/ai-provider";

export type AiProviderPreview = {
	view: AiProviderView;
	outcome?: AiProviderOutcome;
};

export const aiProviderPreviewStates: Record<string, AiProviderPreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			unconfigured: { view: { kind: "unconfigured" } },
			untested: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "unverified",
						createdAt: "2026-09-22T09:10:00Z",
						updatedAt: "2026-09-22T09:10:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
			},
			no_model: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "",
						embeddingModel: "",
						status: "unverified",
						createdAt: "2026-09-22T09:10:00Z",
						updatedAt: "2026-09-22T09:10:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
				outcome: { kind: "saved" },
			},
			verified: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "text-embedding-3-small",
						status: "verified",
						verifiedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-22T09:10:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
			},
			key_rejected: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "failed",
						failure: "key_rejected",
						verifiedAt: "2026-09-20T16:40:00Z",
						failedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-18T11:00:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "failed",
						failure: {
							kind: "provider",
							code: "key_rejected",
							detail: "the provider rejected this API key: Incorrect API key provided: sk-proj-********q7Xe.",
						},
					},
				},
				outcome: {
					kind: "failed",
					failure: {
						kind: "provider",
						code: "key_rejected",
						detail: "the provider rejected this API key: Incorrect API key provided: sk-proj-********q7Xe.",
					},
				},
			},
			model_unavailable: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-5.4-nano",
						embeddingModel: "",
						status: "failed",
						failure: "model_unavailable",
						failedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-18T11:00:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
				outcome: {
					kind: "failed",
					failure: {
						kind: "provider",
						code: "model_unavailable",
						detail: "this API key cannot use that model: The model `gpt-5.4-nano` does not exist or you do not have access to it.",
					},
				},
			},
			embedding_model_unavailable: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "verified",
						verifiedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-18T11:00:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
				outcome: {
					kind: "failed",
					failure: {
						kind: "provider",
						code: "embedding_model_unavailable",
						detail: "this API key cannot use that embedding model: The model `text-embedding-3-large` does not exist or you do not have access to it.",
					},
				},
			},
			quota_exceeded: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "failed",
						failure: "quota_exceeded",
						failedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-18T11:00:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
				outcome: {
					kind: "failed",
					failure: {
						kind: "provider",
						code: "quota_exceeded",
						detail: "the provider account behind this API key is out of quota: You exceeded your current quota.",
					},
				},
			},
			rate_limited: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "failed",
						failure: "rate_limited",
						verifiedAt: "2026-09-21T08:00:00Z",
						failedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-18T11:00:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
				outcome: {
					kind: "failed",
					failure: { kind: "provider", code: "rate_limited" },
				},
			},
			unreachable: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "failed",
						failure: "unreachable",
						verifiedAt: "2026-09-21T08:00:00Z",
						failedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-18T11:00:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "failed",
						failure: { kind: "provider", code: "unreachable" },
					},
				},
				outcome: {
					kind: "failed",
					failure: { kind: "provider", code: "unreachable" },
				},
			},
			custom_endpoint: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "http://10.0.4.12:8000/v1",
						allowPrivateAddress: true,
						keyHint: "local",
						defaultModel: "llama-4-scout",
						embeddingModel: "nomic-embed-text",
						status: "verified",
						verifiedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-22T09:10:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "listed",
						models: ["llama-4-maverick", "llama-4-scout", "qwen3-coder"],
						embeddingModels: ["nomic-embed-text"],
					},
				},
			},
			destination_refused: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "http://127.0.0.1:11434/v1",
						allowPrivateAddress: true,
						keyHint: "llam",
						defaultModel: "llama-4-scout",
						embeddingModel: "",
						status: "failed",
						failure: "destination_refused",
						failedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-22T09:10:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "failed",
						failure: { kind: "provider", code: "destination_refused" },
					},
				},
				outcome: {
					kind: "failed",
					failure: {
						kind: "provider",
						code: "destination_refused",
						detail: "this instance will not open a connection to that endpoint: 127.0.0.1 is a loopback address",
					},
				},
			},
			sealing_unavailable: {
				view: { kind: "unconfigured" },
				outcome: { kind: "failed", failure: { kind: "sealing_unavailable" } },
			},
			key_unreadable: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "verified",
						verifiedAt: "2026-09-21T08:00:00Z",
						createdAt: "2026-09-18T11:00:00Z",
						updatedAt: "2026-09-21T08:00:00Z",
					},
					models: { kind: "failed", failure: { kind: "sealing_unavailable" } },
				},
			},
			saved: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "",
						status: "unverified",
						createdAt: "2026-09-22T09:10:00Z",
						updatedAt: "2026-09-22T09:10:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
				outcome: { kind: "saved" },
			},
			tested: {
				view: {
					kind: "configured",
					provider: {
						provider: "openai",
						baseUrl: "",
						allowPrivateAddress: false,
						keyHint: "q7Xe",
						defaultModel: "gpt-6-luna",
						embeddingModel: "text-embedding-3-small",
						status: "verified",
						verifiedAt: "2026-09-22T09:12:00Z",
						createdAt: "2026-09-22T09:10:00Z",
						updatedAt: "2026-09-22T09:12:00Z",
					},
					models: {
						kind: "listed",
						models: ["gpt-6-astra", "gpt-6-luna", "gpt-6-sol", "gpt-oss-120b"],
						embeddingModels: ["text-embedding-3-large", "text-embedding-3-small"],
					},
				},
				outcome: { kind: "tested" },
			},
			removed: {
				view: { kind: "unconfigured" },
				outcome: { kind: "removed" },
			},
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
		}
	: {};
