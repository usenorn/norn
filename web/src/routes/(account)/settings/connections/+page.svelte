<script lang="ts">
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Plug from "@lucide/svelte/icons/plug";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { onDate } from "$lib/time";
	import { narrowConnectionSchema } from "$lib/account/narrow-connection-schema";
	import type { GrantInput } from "$lib/account/mint-token-schema";
	import {
		capabilityLabel,
		connectionFailureMessage,
		reachSummary,
		type ConnectionFailure,
		type ConnectionListing,
		type MCPConnection,
	} from "$lib/account/connections";
	import { connectionsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? connectionsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let replaced = $state<ConnectionListing | null>(null);
	let failure = $state<ConnectionFailure | null>(null);
	let revoking = $state<string | null>(null);
	let narrowing = $state<MCPConnection | null>(null);

	const listing = $derived<ConnectionListing>(replaced ?? preview?.listing ?? data.listing);
	const workspaces = $derived(preview?.workspaces ?? data.workspaces);

	const current = $derived(listing.kind === "ready" ? listing.connections : []);

	const form = superForm(defaults(zod4(narrowConnectionSchema)), {
		SPA: true,
		validators: zod4Client(narrowConnectionSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || !narrowing) return;

			failure = null;

			try {
				const { data: narrowed, error } = await api.PATCH("/mcp/connections/{connectionId}", {
					params: { path: { connectionId: narrowing.id } },
					body: {
						capability: entered.data.capability,
						grants: entered.data.followsMembership
							? undefined
							: entered.data.grants.map((grant) => ({
									workspaceId: grant.workspaceId,
									allTeams: grant.allTeams,
									teamIds: grant.allTeams ? undefined : grant.teamIds,
								})),
					},
				});

				if (error || !narrowed) {
					failure =
						error?.status === 422
							? { kind: "grant_invalid" }
							: error?.status === 403
								? { kind: "forbidden" }
								: { kind: "unavailable" };

					return;
				}

				replaced = {
					kind: "ready",
					connections: current.map((connection) =>
						connection.id === narrowed.id ? narrowed : connection
					),
				};
				narrowing = null;
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	const busy = $derived(preview?.busy || $submitting || revoking !== null);

	function beginNarrowing(connection: MCPConnection) {
		narrowing = connection;
		failure = null;

		formData.set({
			capability: connection.capability,
			followsMembership: connection.followsMembership,
			grants: connection.followsMembership
				? workspaces.map((workspace) => ({
						workspaceId: workspace.id,
						allTeams: true,
						teamIds: [],
					}))
				: connection.grants.map((grant) => ({
						workspaceId: grant.workspaceId,
						allTeams: grant.allTeams,
						teamIds: grant.teamIds ?? [],
					})),
		});
	}

	function grantFor(workspaceId: string): GrantInput | undefined {
		return $formData.grants.find((grant) => grant.workspaceId === workspaceId);
	}

	function narrowableWorkspaces(connection: MCPConnection) {
		if (connection.followsMembership) return workspaces;

		return workspaces.filter((workspace) =>
			connection.grants.some((grant) => grant.workspaceId === workspace.id)
		);
	}

	function toggleWorkspace(workspaceId: string, checked: boolean) {
		formData.update((entered) => ({
			...entered,
			followsMembership: false,
			grants: checked
				? [...entered.grants, { workspaceId, allTeams: true, teamIds: [] }]
				: entered.grants.filter((grant) => grant.workspaceId !== workspaceId),
		}));
	}

	function setAllTeams(workspaceId: string, allTeams: boolean) {
		formData.update((entered) => ({
			...entered,
			followsMembership: false,
			grants: entered.grants.map((grant) =>
				grant.workspaceId === workspaceId ? { ...grant, allTeams, teamIds: [] } : grant
			),
		}));
	}

	function toggleTeam(workspaceId: string, teamId: string, checked: boolean) {
		formData.update((entered) => ({
			...entered,
			followsMembership: false,
			grants: entered.grants.map((grant) => {
				if (grant.workspaceId !== workspaceId) return grant;

				const teamIds = checked
					? [...grant.teamIds, teamId]
					: grant.teamIds.filter((id) => id !== teamId);

				return { ...grant, teamIds };
			}),
		}));
	}

	async function revoke(connectionId: string) {
		revoking = connectionId;
		failure = null;

		try {
			const { error } = await api.DELETE("/mcp/connections/{connectionId}", {
				params: { path: { connectionId } },
			});

			if (error) {
				failure = error.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };

				return;
			}

			const remaining = current.filter((connection) => connection.id !== connectionId);

			replaced =
				remaining.length === 0 ? { kind: "empty" } : { kind: "ready", connections: remaining };

			if (narrowing?.id === connectionId) narrowing = null;
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			revoking = null;
		}
	}

	function formatted(instant: string | undefined): string | null {
		if (!instant) return null;

		return onDate(instant, "UTC");
	}

	function reaches(connection: MCPConnection): string {
		if (connection.followsMembership) {
			return "every workspace you belong to, including ones you join later";
		}

		return connection.grants
			.map((grant) => {
				const workspace = workspaces.find((candidate) => candidate.id === grant.workspaceId);
				const name = workspace?.name ?? "a workspace you have left";

				if (grant.allTeams) return name;

				return `${name} (${grant.teamIds?.length ?? 0} team${grant.teamIds?.length === 1 ? "" : "s"})`;
			})
			.join(" · ");
	}
</script>

<svelte:head><title>AI clients · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center border-b border-line-subtle px-4">
		<h1 class="text-sm font-medium tracking-snug text-ink-900">AI clients</h1>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{connectionFailureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if listing.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not manage connections</Alert.Title>
					<Alert.Description>Connections belong to the person who approved them.</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load your connections</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading your connections…</p>
			{:else}
				<div class="flex flex-col gap-1">
					<h2 class="text-md font-medium tracking-snug text-ink-900">Connected AI clients</h2>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Each connection acts as you, across every workspace it reaches. Narrowing shrinks a
						connection without breaking it; revoking cuts it off from its next request.
					</p>
				</div>

				{#if current.length === 0}
					<p class="text-sm text-muted-foreground">
						No AI clients are connected. A client connects by opening Norn's MCP server and
						asking for your approval.
					</p>
				{:else}
					<ul class="rounded-lg border border-line-subtle bg-paper-0">
						{#each current as connection (connection.id)}
							<li class="flex flex-col gap-2 border-b border-line-subtle p-3 last:border-b-0">
								<div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
									<div class="flex min-w-0 flex-col gap-1.5">
										<div class="flex items-center gap-2">
											<Plug class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
											<span class="truncate text-sm text-ink-900">{connection.clientName}</span>
											<Tag name={capabilityLabel[connection.capability]} />
										</div>
										<p class="text-xs text-muted-foreground">Reaches {reaches(connection)}</p>
										<p class="text-xs text-muted-foreground">
											Connected {formatted(connection.createdAt)}
											{#if connection.lastUsedAt}&middot; Last used {formatted(
													connection.lastUsedAt
												)}{/if}
										</p>
									</div>
									<div class="flex flex-none gap-2">
										<Button
											variant="secondary"
											size="sm"
											disabled={busy}
											onclick={() =>
												narrowing?.id === connection.id
													? (narrowing = null)
													: beginNarrowing(connection)}
										>
											{narrowing?.id === connection.id ? "Close" : "Narrow"}
										</Button>
										<Button
											variant="secondary"
											size="sm"
											disabled={busy}
											onclick={() => revoke(connection.id)}
										>
											{revoking === connection.id ? "Revoking…" : "Revoke"}
										</Button>
									</div>
								</div>

								{#if narrowing?.id === connection.id}
									<form
										method="POST"
										use:enhance
										class="flex flex-col gap-4 rounded-md border border-line-subtle p-3"
									>
										<p class="text-sm leading-normal text-muted-foreground text-pretty">
											Narrowing only ever shrinks what {connection.clientName} reaches. To widen
											it again, revoke and re-connect.
										</p>

										{#if connection.capability === "write"}
											<Form.Field {form} name="capability">
												<Form.Control>
													{#snippet children({ props })}
														<div class="flex items-center gap-2">
															<Checkbox
																{...props}
																id={`read-only-${connection.id}`}
																disabled={busy}
																checked={$formData.capability === "read"}
																onCheckedChange={(checked) =>
																	formData.update((entered) => ({
																		...entered,
																		capability: checked === true ? "read" : "write",
																	}))}
															/>
															<label
																for={`read-only-${connection.id}`}
																class="text-sm leading-normal text-ink-900"
															>
																Make it read-only
															</label>
														</div>
													{/snippet}
												</Form.Control>
												<Form.FieldErrors />
											</Form.Field>
										{/if}

										<Form.Field {form} name="grants">
											<Form.Control>
												{#snippet children({ props })}
													<Form.Label>Where it reaches</Form.Label>
													<div {...props} class="flex flex-col gap-3">
														{#each narrowableWorkspaces(connection) as workspace (workspace.id)}
															{@const grant = grantFor(workspace.id)}
															<div class="rounded-md border border-line-subtle p-3">
																<div class="flex items-start gap-2">
																	<Checkbox
																		id={`connection-workspace-${workspace.id}`}
																		disabled={busy}
																		checked={grant !== undefined}
																		onCheckedChange={(checked) =>
																			toggleWorkspace(workspace.id, checked === true)}
																	/>
																	<label
																		for={`connection-workspace-${workspace.id}`}
																		class="text-sm leading-normal text-ink-900"
																	>
																		{workspace.name}
																	</label>
																</div>

																{#if grant}
																	<div class="mt-3 flex flex-col gap-2 pl-6">
																		<div class="flex items-center gap-2">
																			<Checkbox
																				id={`connection-all-teams-${workspace.id}`}
																				disabled={busy}
																				checked={grant.allTeams}
																				onCheckedChange={(checked) =>
																					setAllTeams(workspace.id, checked === true)}
																			/>
																			<label
																				for={`connection-all-teams-${workspace.id}`}
																				class="text-sm leading-normal text-muted-foreground"
																			>
																				Every team you can see
																			</label>
																		</div>

																		{#if !grant.allTeams}
																			{@const teams = data.teams[workspace.id] ?? []}
																			{#if teams.length === 0}
																				<p class="text-xs text-muted-foreground">
																					No teams you can see in this workspace.
																				</p>
																			{:else}
																				<div class="flex flex-col gap-1.5">
																					{#each teams as team (team.id)}
																						<div class="flex items-center gap-2">
																							<Checkbox
																								id={`connection-team-${workspace.id}-${team.id}`}
																								disabled={busy}
																								checked={grant.teamIds.includes(team.id)}
																								onCheckedChange={(checked) =>
																									toggleTeam(
																										workspace.id,
																										team.id,
																										checked === true
																									)}
																							/>
																							<label
																								for={`connection-team-${workspace.id}-${team.id}`}
																								class="text-sm leading-normal text-ink-600"
																							>
																								{team.name}
																							</label>
																						</div>
																					{/each}
																				</div>
																			{/if}
																		{/if}
																	</div>
																{/if}
															</div>
														{/each}
													</div>
												{/snippet}
											</Form.Control>
											<Form.FieldErrors />
										</Form.Field>

										{#if $formData.followsMembership}
											<p class="text-xs leading-normal text-muted-foreground text-pretty">
												Right now it follows your membership. Unticking a workspace pins the
												connection to the ones left ticked — workspaces you join later will no
												longer be included.
											</p>
										{/if}

										<div>
											<Form.Button size="sm" disabled={busy || $formData.followsMembership}>
												{$submitting ? "Narrowing…" : "Narrow connection"}
											</Form.Button>
										</div>
									</form>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			{/if}
		</div>
	</div>
</div>
