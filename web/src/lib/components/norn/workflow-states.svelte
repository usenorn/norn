<script lang="ts">
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronUp from "@lucide/svelte/icons/chevron-up";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Plus from "@lucide/svelte/icons/plus";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import StatusIcon from "./status-icon.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import {
		categoryHints,
		categoryLabels,
		conflictFailure,
		removalBlocker,
		reordered,
		stateCategories,
		stateFailureMessage,
		statesOf,
		type StateCategory,
		type StateFailure,
		type StateList,
		type WorkflowState,
	} from "$lib/team/states";
	import { workflowStateSchema } from "$lib/team/workflow-state-schema";
	import type { Team } from "$lib/team/teams";
	import { useToasts } from "$lib/toast/toasts.svelte";

	const formId = "workflow-state-form";

	let {
		workspaceId,
		team,
		list,
		locked = false,
	}: { workspaceId: string; team: Team; list: StateList; locked?: boolean } = $props();

	const toasts = useToasts();

	let submitted = $state<WorkflowState[] | null>(null);
	let failure = $state<StateFailure | null>(null);
	let editingId = $state("");
	let working = $state("");
	let removing = $state(false);
	let reassignTo = $state("");

	const states = $derived(submitted ?? statesOf(list));
	const removalId = $derived(page.url.searchParams.get("remove") ?? "");
	const removalState = $derived(states.find((state) => state.id === removalId) ?? null);
	const blocker = $derived(removalState ? removalBlocker(states, removalState) : null);
	const alternatives = $derived(states.filter((state) => state.id !== removalId));
	const editing = $derived(states.find((state) => state.id === editingId) ?? null);

	const form = superForm(defaults(zod4(workflowStateSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(workflowStateSchema),
		resetForm: false,
		onUpdate: async ({ form: pendingForm }) => {
			if (!pendingForm.valid) return;

			failure = null;
			const body = { name: pendingForm.data.name, category: pendingForm.data.category };

			try {
				const result = editing
					? await api.PATCH("/workspaces/{workspaceId}/teams/{teamId}/states/{stateId}", {
							params: { path: { workspaceId, teamId: team.id, stateId: editing.id } },
							body,
						})
					: await api.POST("/workspaces/{workspaceId}/teams/{teamId}/states", {
							params: { path: { workspaceId, teamId: team.id } },
							body,
						});

				if (result.data) {
					toasts.show(editing
						? `${result.data.name} was saved.`
						: `${result.data.name} was added.`);
					stopEditing();
					await refresh();

					return;
				}

				const problem = result.error;

				if (problem && "code" in problem && typeof problem.code === "string") {
					const conflict = conflictFailure(problem.code, editing ?? states[0]);

					if (conflict?.kind === "name_taken") {
						setError(pendingForm, "name", stateFailureMessage(conflict));

						return;
					}

					if (conflict) {
						failure = conflict;

						return;
					}
				}

				if (problem?.status === 403) {
					failure = { kind: "forbidden" };

					return;
				}

				failure = { kind: "unavailable" };
			} catch {
				failure = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	const busy = $derived(locked || $submitting || working !== "" || removing);

	async function refresh() {
		const { data } = await api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
			params: { path: { workspaceId, teamId: team.id } },
		});

		if (data) submitted = data;

		await invalidate(keys.page(page.route.id));
	}

	function startEditing(state: WorkflowState) {
		editingId = state.id;
		failure = null;
		formData.set({ name: state.name, category: state.category }, { taint: false });
	}

	function stopEditing() {
		editingId = "";
		formData.set({ name: "", category: "not_started" }, { taint: false });
	}

	async function move(state: WorkflowState, delta: number) {
		const stateIds = reordered(states, state.id, delta);
		if (stateIds.every((id, index) => id === states[index].id)) return;

		working = state.id;
		failure = null;

		try {
			const { data, error } = await api.PUT(
				"/workspaces/{workspaceId}/teams/{teamId}/states/order",
				{ params: { path: { workspaceId, teamId: team.id } }, body: { stateIds } }
			);

			if (data) {
				submitted = data;
				toasts.show(`${state.name} moved to position ${stateIds.indexOf(state.id) + 1} of ${stateIds.length}.`);
				await invalidate(keys.page(page.route.id));

				return;
			}

			failure = error?.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	async function promote(state: WorkflowState, role: "default" | "completion") {
		working = state.id;
		failure = null;

		try {
			const path = { workspaceId, teamId: team.id, stateId: state.id };
			const { data, error } =
				role === "default"
					? await api.POST(
							"/workspaces/{workspaceId}/teams/{teamId}/states/{stateId}/default",
							{ params: { path } }
						)
					: await api.POST(
							"/workspaces/{workspaceId}/teams/{teamId}/states/{stateId}/completion",
							{ params: { path } }
						);

			if (data) {
				submitted = data;
				toasts.show(
					role === "default"
						? `New issues now start in ${state.name}.`
						: `${state.name} now counts as finished.`
				);
				await invalidate(keys.page(page.route.id));

				return;
			}

			if (error && "code" in error && typeof error.code === "string") {
				const conflict = conflictFailure(error.code, state);

				if (conflict) {
					failure = conflict;

					return;
				}
			}

			failure = error?.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	function removeHref(state: WorkflowState): string {
		const next = new URL(page.url);
		next.searchParams.set("remove", state.id);

		return `${next.pathname}${next.search}`;
	}

	async function closeRemoval() {
		const next = new URL(page.url);
		next.searchParams.delete("remove");
		reassignTo = "";

		await goto(next, { replaceState: true, noScroll: true });
	}

	async function confirmRemoval() {
		if (!removalState || !reassignTo) return;

		removing = true;
		failure = null;

		try {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/teams/{teamId}/states/{stateId}",
				{
					params: {
						path: { workspaceId, teamId: team.id, stateId: removalState.id },
						query: { replacementStateId: reassignTo },
					},
				}
			);

			if (error) {
				if ("code" in error && typeof error.code === "string") {
					const conflict = conflictFailure(error.code, removalState);

					if (conflict) {
						failure = conflict;

						return;
					}
				}

				failure = error.status === 403 ? { kind: "forbidden" } : { kind: "unavailable" };

				return;
			}

			const target = states.find((state) => state.id === reassignTo);
			toasts.show(`${removalState.name} was removed. Its issues moved to ${target?.name ?? "another state"}.`);
			await closeRemoval();
			await refresh();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			removing = false;
		}
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">States</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			The steps an issue moves through in {team.name}. Name them however this team talks; the four
			categories are what let progress add up across teams that use different words.
		</p>
	</div>

	{#if failure}
		<Alert.Root variant="destructive">
			<CircleX aria-hidden="true" />
			<Alert.Title>That did not work</Alert.Title>
			<Alert.Description>{stateFailureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if list.kind === "loading"}
		<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
	{:else if list.kind === "unavailable"}
		<p class="text-sm leading-normal text-muted-foreground">
			We could not load this team's states.
		</p>
	{:else}
		<ol class="flex flex-col rounded-lg border border-line-default">
			{#each states as state, index (state.id)}
				{@const blocked = removalBlocker(states, state)}
				<li class="flex flex-col border-b border-line-subtle last:border-b-0">
					<div class="flex flex-wrap items-center gap-2 px-3 py-2">
						<StatusIcon category={state.category} name={state.name} />
						<span class="min-w-0 flex-[1_1_120px] truncate text-md text-ink-900">{state.name}</span>

						{#if state.isDefault}
							<span
								class="shrink-0 rounded-sm bg-paper-2 px-1.5 py-0.5 font-mono text-2xs tracking-eyebrow text-ink-600 uppercase"
							>
								Default
							</span>
						{/if}
						{#if state.isCompletion}
							<span
								class="shrink-0 rounded-sm bg-paper-2 px-1.5 py-0.5 font-mono text-2xs tracking-eyebrow text-ink-600 uppercase"
							>
								Completion
							</span>
						{/if}

						<span class="shrink-0 text-sm text-muted-foreground">
							{categoryLabels[state.category]}
						</span>

						<span class="flex shrink-0 items-center gap-0.5">
							<Button
								variant="ghost"
								size="icon-sm"
								disabled={busy || index === 0}
								aria-label="Move {state.name} up"
								onclick={() => move(state, -1)}
							>
								<ChevronUp aria-hidden="true" />
							</Button>
							<Button
								variant="ghost"
								size="icon-sm"
								disabled={busy || index === states.length - 1}
								aria-label="Move {state.name} down"
								onclick={() => move(state, 1)}
							>
								<ChevronDown aria-hidden="true" />
							</Button>
							<Button
								variant="ghost"
								size="sm"
								disabled={busy}
								onclick={() => (editingId === state.id ? stopEditing() : startEditing(state))}
							>
								{editingId === state.id ? "Cancel" : "Edit"}
							</Button>
							<Button
								variant="ghost"
								size="icon-sm"
								disabled={busy || blocked !== null}
								aria-label={blocked
									? `Cannot remove ${state.name}: ${stateFailureMessage(blocked)}`
									: `Remove ${state.name}`}
								title={blocked ? stateFailureMessage(blocked) : undefined}
								href={removeHref(state)}
							>
								<X aria-hidden="true" />
							</Button>
						</span>
					</div>

					{#if blocked}
						<p class="px-3 pb-2 text-sm leading-normal text-muted-foreground text-pretty">
							{stateFailureMessage(blocked)}
						</p>
					{/if}

					{#if removalId === state.id}
						<div class="flex flex-col gap-3 border-t border-line-subtle bg-paper-1 px-3 py-3">
							{#if blocker}
								<p class="text-sm leading-normal text-muted-foreground text-pretty">
									{stateFailureMessage(blocker)}
								</p>
								<div>
									<Button variant="secondary" size="sm" onclick={closeRemoval}>Close</Button>
								</div>
							{:else}
								<div class="flex flex-col gap-1">
									<span class="text-sm font-medium text-ink-900">
										Remove {state.name}
									</span>
									<p class="text-sm leading-normal text-muted-foreground text-pretty">
										Choose where its issues go. Every issue in {state.name} moves there, and what
										already happened to them stays in their history.
									</p>
								</div>

								<Select.Root
									type="single"
									value={reassignTo}
									disabled={removing}
									onValueChange={(value) => (reassignTo = value)}
								>
									<Select.Trigger aria-label="Move issues to">
										{alternatives.find((candidate) => candidate.id === reassignTo)?.name ??
											"Choose a state"}
									</Select.Trigger>
									<Select.Content>
										{#each alternatives as candidate (candidate.id)}
											<Select.Item value={candidate.id} label={candidate.name}>
												{candidate.name}
											</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>

								<div class="flex gap-2">
									<Button
										variant="destructive"
										size="sm"
										disabled={removing || !reassignTo}
										onclick={confirmRemoval}
									>
										{removing ? "Removing" : "Remove and move issues"}
									</Button>
									<Button variant="secondary" size="sm" disabled={removing} onclick={closeRemoval}>
										Keep it
									</Button>
								</div>
							{/if}
						</div>
					{/if}

					{#if !state.isDefault || !state.isCompletion}
						<div class="flex flex-wrap gap-2 px-3 pb-2">
							{#if !state.isDefault}
								<Button
									variant="ghost"
									size="sm"
									disabled={busy}
									onclick={() => promote(state, "default")}
								>
									Start new issues here
								</Button>
							{/if}
							{#if !state.isCompletion && state.category === "complete"}
								<Button
									variant="ghost"
									size="sm"
									disabled={busy}
									onclick={() => promote(state, "completion")}
								>
									Count this as finished
								</Button>
							{/if}
						</div>
					{/if}
				</li>
			{/each}
		</ol>

		<form id={formId} method="POST" use:enhance class="flex flex-col gap-4">
			<div class="flex flex-col gap-1">
				<h3 class="text-sm font-medium text-ink-900">
					{editing ? `Edit ${editing.name}` : "Add a state"}
				</h3>
			</div>

			<Form.Field {form} name="name">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Name</Form.Label>
						<Input
							{...props}
							disabled={busy}
							placeholder="Ready for review"
							bind:value={$formData.name}
						/>
					{/snippet}
				</Form.Control>
				<Form.FieldErrors />
			</Form.Field>

			<Form.Field {form} name="category">
				<Form.Control>
					{#snippet children({ props })}
						<Form.Label>Category</Form.Label>
						<Select.Root
							type="single"
							value={$formData.category}
							disabled={busy}
							onValueChange={(value) => ($formData.category = value as StateCategory)}
						>
							<Select.Trigger {...props}>{categoryLabels[$formData.category]}</Select.Trigger>
							<Select.Content>
								{#each stateCategories as category (category)}
									<Select.Item value={category} label={categoryLabels[category]}>
										{categoryLabels[category]}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					{/snippet}
				</Form.Control>
				<Form.Description class="text-sm text-muted-foreground">
					{categoryHints[$formData.category]}
				</Form.Description>
				<Form.FieldErrors />
			</Form.Field>
		</form>

		<div class="flex gap-2">
			<Button type="submit" form={formId} disabled={busy}>
				{#if !editing}
					<Plus aria-hidden="true" />
				{/if}
				{$submitting ? "Saving" : editing ? "Save state" : "Add state"}
			</Button>
			{#if editing}
				<Button variant="secondary" disabled={busy} onclick={stopEditing}>Cancel</Button>
			{/if}
		</div>
	{/if}
</section>
