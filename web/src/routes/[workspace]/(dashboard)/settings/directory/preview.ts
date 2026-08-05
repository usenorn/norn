import type { DirectoryFailure, DirectoryView } from "$lib/directory/directory";

export type DirectoryPreview = {
	view?: DirectoryView;
	failure?: DirectoryFailure;
	busy?: boolean;
};

export const directoryPreviewStates: Record<string, DirectoryPreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			unlicensed: { view: { kind: "unlicensed" } },
			disconnected: { view: { kind: "disconnected" } },
			connecting: { view: { kind: "disconnected" }, busy: true },
			issued: {
				view: {
					kind: "issued",
					token: "nrnscim_R0xCa1ZoWmZKUlhkQlVvbmVUaW1lT25seQ",
					scimBaseUrl: "/v1/scim/v2",
					connection: {
						enabled: true,
						onUnknown: "provision",
						onAbsent: "deactivate",
						adminGroup: "norn-admins",
					},
					runs: [],
				},
			},
			connected: {
				view: {
					kind: "connected",
					scimBaseUrl: "/v1/scim/v2",
					connection: {
						enabled: true,
						onUnknown: "provision",
						onAbsent: "deactivate",
						adminGroup: "norn-admins",
						lastSyncAt: "2026-08-05T09:14:22Z",
					},
					runs: [
						{
							id: "00000000-0000-4000-8000-00000000b001",
							trigger: "scim",
							outcome: "succeeded",
							operation: "PutUser",
							startedAt: "2026-08-05T09:14:22Z",
							finishedAt: "2026-08-05T09:14:22Z",
						},
						{
							id: "00000000-0000-4000-8000-00000000b002",
							trigger: "scim",
							outcome: "refused",
							operation: "DeleteUser",
							startedAt: "2026-08-05T08:51:03Z",
							finishedAt: "2026-08-05T08:51:03Z",
							detail: "workspace would be left without an administrator",
						},
						{
							id: "00000000-0000-4000-8000-00000000b003",
							trigger: "scim",
							outcome: "failed",
							operation: "EditGroupMembers",
							startedAt: "2026-08-05T08:12:47Z",
							finishedAt: "2026-08-05T08:12:47Z",
							detail: "the directory does not know this user here",
						},
					],
				},
			},
			empty_log: {
				view: {
					kind: "connected",
					scimBaseUrl: "/v1/scim/v2",
					connection: {
						enabled: true,
						onUnknown: "provision",
						onAbsent: "deactivate",
						adminGroup: "",
					},
					runs: [],
				},
			},
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
		}
	: {};
