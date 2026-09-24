<script lang="ts">
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import PackageSearch from "@lucide/svelte/icons/package-search";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { page } from "$app/state";
	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import * as Tabs from "$lib/components/ui/tabs/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import McpVariableFields from "./mcp-variable-fields.svelte";
	import {
		capabilityFailure,
		capabilityFailureMessage,
		fieldProblems,
		mcpServerTarget,
		serverFieldMessage,
		type AgentCapabilityFailure,
		type AgentMcpServer,
		type McpRegistryEntry,
		type McpServerDialogMode,
		type McpServerTemplate,
	} from "./agent-capabilities";
	import {
		emptyMcpServer,
		mcpServerInput,
		mcpServerRequest,
		mcpServerSchema,
		registryServerInput,
	} from "./agent-capability-schemas";

	let {
		open = $bindable(false),
		workspaceId,
		agentId,
		mode,
		selfHosted,
		onsaved,
	}: {
		open?: boolean;
		workspaceId: string;
		agentId?: string;
		mode: McpServerDialogMode;
		selfHosted: boolean;
		onsaved: (server: AgentMcpServer) => void;
	} = $props();

	type Search =
		| { kind: "idle" }
		| { kind: "searching" }
		| { kind: "found"; entries: McpRegistryEntry[] }
		| { kind: "failed"; failure: AgentCapabilityFailure };

	const searchDelay = 300;

	let tab = $state<"registry" | "configure">("registry");
	let query = $state("");
	let search = $state.raw<Search>({ kind: "idle" });
	let failure = $state<AgentCapabilityFailure | null>(null);
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let searchRound = 0;

	const form = superForm(defaults(emptyMcpServer(), zod4(mcpServerSchema)), {
		id: "mcp-server-form",
		SPA: true,
		dataType: "json",
		validators: zod4Client(mcpServerSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = null;

			const body = mcpServerRequest(entered.data);

			try {
				const { data, error, response } =
					mode.kind === "edit"
						? await api.PUT("/workspaces/{workspaceId}/agent-mcp-servers/{serverId}", {
								params: { path: { workspaceId, serverId: mode.server.id } },
								body,
							})
						: agentId
							? await api.POST("/workspaces/{workspaceId}/agents/{agentId}/mcp-servers", {
									params: { path: { workspaceId, agentId } },
									body,
								})
							: await api.POST("/workspaces/{workspaceId}/agent-library/mcp-servers", {
									params: { path: { workspaceId } },
									body,
								});

				if (error || !data) {
					const problems = fieldProblems(error);

					for (const problem of problems) {
						if (problem.field in entered.data) {
							setError(entered, problem.field as "name", serverFieldMessage(problem));
						}
					}

					if (problems.length === 0) failure = capabilityFailure(error, response.status);

					return;
				}

				onsaved(data);
				open = false;
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});

	const { form: formData, enhance, submitting } = form;

	const remote = $derived($formData.transport !== "stdio");
	const editing = $derived(mode.kind === "edit");

	$effect(() => {
		if (!open) return;

		failure = null;
		query = "";
		search = { kind: "idle" };
		tab = mode.kind === "edit" ? "configure" : "registry";
		form.reset({ keepMessage: false, data: mode.kind === "edit" ? mcpServerInput(mode.server) : emptyMcpServer() });

		if (mode.kind === "add") void find("");
	});

	function typed(value: string) {
		query = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void find(value), searchDelay);
	}

	async function find(value: string) {
		const round = ++searchRound;
		search = { kind: "searching" };

		try {
			const { data, error, response } = await api.GET("/workspaces/{workspaceId}/mcp-registry", {
				params: { path: { workspaceId }, query: { search: value.trim() || undefined } },
			});

			if (round !== searchRound) return;

			search =
				error || !data
					? { kind: "failed", failure: capabilityFailure(error, response.status) }
					: { kind: "found", entries: data };
		} catch {
			if (round === searchRound) search = { kind: "failed", failure: { kind: "unavailable" } };
		}
	}

	function choose(entry: McpRegistryEntry, template: McpServerTemplate) {
		form.reset({ keepMessage: false, data: registryServerInput(entry, template) });
		tab = "configure";
	}

	function templateLabel(template: McpServerTemplate): string {
		if (template.transport !== "stdio") return template.transport === "sse" ? "Remote · SSE" : "Remote · HTTP";

		return `Runs with ${template.command}`;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content variant="scrollable" class="sm:max-w-150">
		<Dialog.Header>
			<Dialog.Title>{editing && mode.kind === "edit" ? `Edit ${mode.server.name}` : "Add an MCP server"}</Dialog.Title>
			<Dialog.Description>
				{agentId
					? "The agent can call the server's tools during a run."
					: "Library servers can be given to any agent in this workspace."}
			</Dialog.Description>
		</Dialog.Header>

		{#if failure}
			<Alert.Root variant="destructive">
				<CircleAlert aria-hidden="true" />
				<Alert.Title>The server was not saved</Alert.Title>
				<Alert.Description>{capabilityFailureMessage(failure)}</Alert.Description>
			</Alert.Root>
		{/if}

		<Tabs.Root bind:value={tab} class="gap-4">
			{#if !editing}
				<Tabs.List variant="line" class="w-full justify-start">
					<Tabs.Trigger value="registry" class="flex-none" disabled={$submitting}>MCP registry</Tabs.Trigger>
					<Tabs.Trigger value="configure" class="flex-none" disabled={$submitting}>Configure</Tabs.Trigger>
				</Tabs.List>
			{/if}

			<Tabs.Content value="registry" class="flex flex-col gap-3">
				<div class="flex flex-col gap-1.5">
					<label for="mcp-registry-search" class="text-sm font-medium text-ink-900">Search the registry</label>
					<Input
						id="mcp-registry-search"
						type="search"
						value={query}
						oninput={(event) => typed(event.currentTarget.value)}
						placeholder="linear, sentry, playwright…"
						autocomplete="off"
					/>
					<p class="text-xs text-muted-foreground">
						Servers published to the official MCP registry. Pick one to fill in how it runs, then review it.
					</p>
				</div>

				<div aria-live="polite" aria-busy={search.kind === "searching"}>
					{#if search.kind === "searching"}
						<div class="flex flex-col gap-2" role="status" aria-label="Searching the registry">
							{#each Array(3) as _}
								<Skeleton class="h-16 w-full" />
							{/each}
						</div>
					{:else if search.kind === "failed"}
						<Alert.Root variant="muted">
							<CircleAlert aria-hidden="true" />
							<Alert.Title>The registry did not answer</Alert.Title>
							<Alert.Description>
								{capabilityFailureMessage(search.failure)} You can still configure a server by hand.
							</Alert.Description>
						</Alert.Root>
					{:else if search.kind === "found" && search.entries.length === 0}
						<p class="flex items-center gap-2 py-6 text-sm text-muted-foreground">
							<PackageSearch class="size-4" aria-hidden="true" />
							Nothing in the registry matches “{query}”.
						</p>
					{:else if search.kind === "found"}
						<ul class="max-h-96 divide-y divide-line-subtle overflow-auto border border-line-subtle bg-paper-1">
							{#each search.entries as entry (entry.name)}
								<li class="flex flex-col gap-2 p-3">
									<div class="min-w-0">
										<div class="flex flex-wrap items-center gap-2">
											<p class="text-sm text-ink-900">{entry.title || entry.name}</p>
											<span class="font-mono text-2xs text-muted-foreground">v{entry.version}</span>
										</div>
										<p class="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{entry.description}</p>
										<p class="mt-0.5 font-mono text-2xs break-all text-muted-foreground">{entry.name}</p>
									</div>
									<div class="flex flex-wrap gap-2">
										{#each entry.templates as template, index (index)}
											<Button type="button" variant="secondary" size="sm" onclick={() => choose(entry, template)}>
												{templateLabel(template)}
											</Button>
										{/each}
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</div>
			</Tabs.Content>

			<Tabs.Content value="configure">
				<form method="POST" id="mcp-server-form" use:enhance class="flex flex-col gap-5">
					{#if $formData.registryName}
						<p class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
							<Tag name="Registry" />
							<span class="font-mono break-all">{$formData.registryName} v{$formData.registryVersion}</span>
						</p>
					{/if}

					<Form.Field {form} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Name</Form.Label>
								<Input
									{...props}
									bind:value={$formData.name}
									disabled={$submitting}
									placeholder="linear"
									autocomplete="off"
									spellcheck="false"
								/>
								<Form.Description>The agent sees its tools as mcp__{$formData.name || "name"}__tool.</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Fieldset {form} name="transport">
						<Form.Legend>How it runs</Form.Legend>
						<RadioGroup.Root name="transport" bind:value={$formData.transport} disabled={$submitting}>
							<div class="flex items-center gap-2">
								<RadioGroup.Item id="mcp-transport-http" value="http" />
								<label for="mcp-transport-http" class="text-sm text-ink-600">Remote server · Streamable HTTP</label>
							</div>
							<div class="flex items-center gap-2">
								<RadioGroup.Item id="mcp-transport-sse" value="sse" />
								<label for="mcp-transport-sse" class="text-sm text-ink-600">Remote server · SSE</label>
							</div>
							<div class="flex items-center gap-2">
								<RadioGroup.Item id="mcp-transport-stdio" value="stdio" />
								<label for="mcp-transport-stdio" class="text-sm text-ink-600">Command on the runner · stdio</label>
							</div>
						</RadioGroup.Root>
						<Form.FieldErrors />
					</Form.Fieldset>

					{#if remote}
						<Form.Field {form} name="url">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Server address</Form.Label>
									<Input
										{...props}
										bind:value={$formData.url}
										disabled={$submitting}
										placeholder="https://mcp.linear.app/mcp"
										autocomplete="url"
										spellcheck="false"
									/>
									<Form.Description>
										{selfHosted
											? "Norn signs in to it from this instance, so it may be on your private network. The runner connects to it during a run."
											: "Norn signs in to it from the cloud, so it must be reachable from the internet. The runner connects to it during a run."}
									</Form.Description>
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Fieldset {form} name="auth">
							<Form.Legend>Sign-in</Form.Legend>
							<RadioGroup.Root name="auth" bind:value={$formData.auth} disabled={$submitting}>
								<div class="flex items-center gap-2">
									<RadioGroup.Item id="mcp-auth-oauth" value="oauth" />
									<label for="mcp-auth-oauth" class="text-sm text-ink-600">OAuth · sign in once from this screen</label>
								</div>
								<div class="flex items-center gap-2">
									<RadioGroup.Item id="mcp-auth-headers" value="headers" />
									<label for="mcp-auth-headers" class="text-sm text-ink-600">Headers · an API key or token</label>
								</div>
								<div class="flex items-center gap-2">
									<RadioGroup.Item id="mcp-auth-none" value="none" />
									<label for="mcp-auth-none" class="text-sm text-ink-600">None</label>
								</div>
							</RadioGroup.Root>
							<Form.FieldErrors />
						</Form.Fieldset>

						{#if $formData.auth === "headers"}
							<McpVariableFields
								{form}
								name="headers"
								legend="Headers"
								description="Sent with every request the run makes to the server. Values are encrypted and never shown again."
								keyPlaceholder="Authorization"
								addLabel="Add header"
								bind:variables={$formData.headers}
								disabled={$submitting}
							/>
						{:else if $formData.auth === "oauth"}
							<details class="group text-sm">
								<summary class="cursor-pointer text-xs text-muted-foreground">
									Use your own OAuth client
								</summary>
								<div class="mt-3 flex flex-col gap-4">
									<p class="text-xs text-muted-foreground text-pretty">
										Only needed when the provider does not let Norn register itself. Register an app with
										redirect URL <code class="font-mono text-2xs break-all">{`${page.url.origin}/v1/agent-mcp/oauth/callback`}</code>.
									</p>
									<Form.Field {form} name="oauthClientId">
										<Form.Control>
											{#snippet children({ props })}
												<Form.Label>Client ID</Form.Label>
												<Input {...props} bind:value={$formData.oauthClientId} disabled={$submitting} autocomplete="off" />
											{/snippet}
										</Form.Control>
										<Form.FieldErrors />
									</Form.Field>
									<Form.Field {form} name="oauthClientSecret">
										<Form.Control>
											{#snippet children({ props })}
												<Form.Label>Client secret</Form.Label>
												<Input
													{...props}
													type="password"
													bind:value={$formData.oauthClientSecret}
													disabled={$submitting}
													placeholder={$formData.oauthClientSecretStored ? "Stored · leave empty to keep" : "Optional"}
													autocomplete="new-password"
												/>
											{/snippet}
										</Form.Control>
										<Form.FieldErrors />
									</Form.Field>
								</div>
							</details>
						{/if}
					{:else}
						<Form.Field {form} name="command">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Command</Form.Label>
									<Input
										{...props}
										bind:value={$formData.command}
										disabled={$submitting}
										placeholder="npx"
										autocomplete="off"
										spellcheck="false"
									/>
									<Form.Description>Runs on the agent's runner, so it must be installed there.</Form.Description>
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="args">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Arguments</Form.Label>
									<Textarea
										{...props}
										bind:value={$formData.args}
										disabled={$submitting}
										rows={3}
										class="font-mono text-xs"
										spellcheck="false"
										placeholder={"-y\n@playwright/mcp@latest"}
									/>
									<Form.Description>One argument per line.</Form.Description>
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<McpVariableFields
							{form}
							name="env"
							legend="Environment"
							description="Set for the command on the runner. Values are encrypted and never shown again."
							keyPlaceholder="API_KEY"
							addLabel="Add variable"
							bind:variables={$formData.env}
							disabled={$submitting}
						/>
					{/if}

					{#if editing && mode.kind === "edit" && mode.server.auth === "oauth" && mode.server.connection}
						<p class="text-xs text-muted-foreground text-pretty">
							Changing the address, the sign-in, or the OAuth client signs {mcpServerTarget(mode.server)} out.
						</p>
					{/if}

					<Dialog.Footer>
						<Button type="button" variant="secondary" disabled={$submitting} onclick={() => (open = false)}>Cancel</Button>
						<Form.Button disabled={$submitting}>
							{$submitting ? "Saving…" : editing ? "Save server" : "Add server"}
						</Form.Button>
					</Dialog.Footer>
				</form>
			</Tabs.Content>
		</Tabs.Root>
	</Dialog.Content>
</Dialog.Root>
