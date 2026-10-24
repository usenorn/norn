<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { api } from "$lib/api";
	import { keys } from "$lib/api/keys";
	import HoldRule from "./hold-rule.svelte";
	import { approvalsPath, type AgentHold, type WorkspaceAgentSettings } from "./agents";

	let {
		workspace,
		settings,
		locked = false,
	}: {
		workspace: { id: string; slug: string; name: string };
		settings: WorkspaceAgentSettings | null;
		locked?: boolean;
	} = $props();

	let saved = $state<WorkspaceAgentSettings | null>(null);
	let failed = $state(false);
	let working = $state(false);

	const current = $derived<WorkspaceAgentSettings>(saved ?? settings ?? { holdDocWrites: "never" });
	const disabled = $derived(locked || working);

	async function save(holdDocWrites: AgentHold) {
		working = true;
		failed = false;

		try {
			const { data, error } = await api.PUT("/workspaces/{workspaceId}/agent-settings", {
				params: { path: { workspaceId: workspace.id } },
				body: { holdDocWrites },
			});

			if (error || !data) {
				failed = true;

				return;
			}

			saved = data;
			await invalidate(keys.page(page.route.id));
		} catch {
			failed = true;
		} finally {
			working = false;
		}
	}
</script>

<section class="flex flex-col gap-4" aria-labelledby="agent-doc-holds-heading">
	<div class="flex flex-col gap-1">
		<h2 id="agent-doc-holds-heading" class="text-md font-semibold tracking-snug text-ink-900">
			Docs
		</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Hold what an agent writes in {workspace.name}'s docs until a person agrees to it. Approving
			applies the change as the agent, so the doc's history still says which one wrote it.
		</p>
	</div>

	{#if failed}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not stick</Alert.Title>
			<Alert.Description>We could not save it. Try again in a moment.</Alert.Description>
		</Alert.Root>
	{/if}

	{#if settings === null}
		<p class="text-sm leading-normal text-muted-foreground">
			We could not read this workspace's agent settings.
		</p>
	{:else}
		<div class="flex flex-col gap-3 rounded-lg border border-line-default p-4">
			<HoldRule
				title="Doc writes"
				detail="A new doc, or an edit to one, waits until somebody has read it."
				options={["never", "always"]}
				value={current.holdDocWrites}
				{disabled}
				onchange={save}
			/>
		</div>

		{#if locked}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Only an administrator of {workspace.name} can change this.
			</p>
		{:else if current.holdDocWrites !== "never"}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Held doc writes wait on the
				<a href={approvalsPath(workspace.slug)} class="text-ink-900 underline underline-offset-2">
					approvals screen
				</a>
				until somebody decides.
			</p>
		{:else}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Nothing is held. An agent that may edit a doc changes it straight away.
			</p>
		{/if}
	{/if}
</section>
