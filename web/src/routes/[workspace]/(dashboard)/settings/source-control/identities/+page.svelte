<script lang="ts">
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Form from "$lib/components/ui/form";
	import { Input } from "$lib/components/ui/input";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import {
		detailOf,
		failureMessage,
		providerLabel,
		sourceControlFailure,
		sourceControlPath,
		type SCMIdentity,
		type SourceControlFailure,
	} from "$lib/source-control/source-control";
	import { mapIdentitySchema } from "$lib/source-control/source-control-schema";
	import { scmIdentityPreviewStates } from "./preview";
	import type { PageData } from "./$types";

	let { data }: { data: PageData } = $props();

	const preview = $derived(
		import.meta.env.DEV
			? scmIdentityPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined,
	);

	let loaded = $state.raw<SCMIdentity[] | undefined>(undefined);
	let failure = $state.raw<SourceControlFailure | undefined>(undefined);
	let failureDetail = $state("");
	let removing = $state("");

	const view = $derived(preview?.view ?? data.view);
	const workspace = $derived(page.data.workspace);

	const identities = $derived(loaded ?? (view.kind === "ready" ? view.identities : []));
	const shown = $derived(failure ?? preview?.failure);

	const memberName = $derived((id: string) => {
		const member = data.members.find((one) => one.id === id);

		return member ? member.name : "Somebody no longer in this workspace";
	});

	const form = superForm(defaults(zod4(mapIdentitySchema)), {
		id: "map-scm-identity",
		SPA: true,
		validators: zod4Client(mapIdentitySchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = undefined;
			failureDetail = "";

			const { data: created, error } = await api.POST(
				"/workspaces/{workspaceId}/source-control/identities",
				{
					params: { path: { workspaceId: workspace.id } },
					body: {
						accountId: entered.data.accountId,
						provider: entered.data.provider,
						login: entered.data.login,
					},
				},
			);

			if (error) {
				const mapped = sourceControlFailure(error);
				const said = detailOf(error, mapped);

				if (mapped.kind === "identity_mapped") {
					setError(entered, "login", said);
				} else {
					failure = mapped;
					failureDetail = said;
				}

				return;
			}

			if (created) loaded = [created, ...identities];
		},
	});

	const { form: fields, enhance, submitting } = form;

	async function unmap(identity: SCMIdentity) {
		removing = identity.id;
		failure = undefined;
		failureDetail = "";

		const { error } = await api.DELETE(
			"/workspaces/{workspaceId}/source-control/identities/{identityId}",
			{ params: { path: { workspaceId: workspace.id, identityId: identity.id } } },
		);

		removing = "";

		if (error) {
			const mapped = sourceControlFailure(error);

			failure = mapped;
			failureDetail = detailOf(error, mapped);

			return;
		}

		loaded = identities.filter((one) => one.id !== identity.id);
	}
</script>

<svelte:head><title>Platform identities · {workspace.name} · Norn</title></svelte:head>

<div class="flex-1 overflow-auto">
	<div
		class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
	>
		<div class="flex flex-col gap-1">
			<Eyebrow>
				<a href={sourceControlPath(workspace.slug)} class="hover:text-ink-900">Source control</a>
			</Eyebrow>
			<h1 class="text-lg font-medium tracking-snug text-ink-900">Platform identities</h1>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Who each member is on each platform. Norn will not guess this: a handle that resembles
				somebody's name is not evidence, and acting on it would assign their work to a stranger.
				Until a person is mapped here, an assignee from the platform is left alone.
			</p>
		</div>

		{#if view.kind === "loading"}
			<p class="text-sm text-muted-foreground">Reading the mapped identities…</p>
		{:else if view.kind === "forbidden"}
			<Alert.Root>
				<Alert.Title>You cannot manage identities</Alert.Title>
				<Alert.Description>{failureMessage({ kind: "forbidden" })}</Alert.Description>
			</Alert.Root>
		{:else if view.kind === "unavailable"}
			<Alert.Root variant="destructive">
				<Alert.Title>Source control could not be reached</Alert.Title>
				<Alert.Description>{failureMessage({ kind: "unavailable" })}</Alert.Description>
			</Alert.Root>
		{:else}
			{#if shown}
				<Alert.Root variant="destructive">
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{failureDetail || failureMessage(shown)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if identities.length === 0}
				<p class="text-sm text-muted-foreground">
					Nobody is mapped yet, so no assignee crosses in either direction.
				</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each identities as identity (identity.id)}
						<li class="flex items-center justify-between gap-3 rounded-md border border-line-subtle px-3 py-2">
							<div class="flex min-w-0 flex-col gap-0.5">
								<p class="truncate text-sm text-ink-900">{memberName(identity.accountId)}</p>
								<p class="truncate text-xs text-muted-foreground">
									{providerLabel(identity.provider)} · {identity.login}
								</p>
							</div>
							<Button
								variant="secondary"
								onclick={() => unmap(identity)}
								disabled={removing === identity.id}
							>
								{removing === identity.id ? "Removing…" : "Unmap"}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}

			<form method="POST" use:enhance class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
				<h2 class="text-md font-medium tracking-snug text-ink-900">Map somebody</h2>

				<Form.Field {form} name="accountId">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Member</Form.Label>
							<select
								{...props}
								bind:value={$fields.accountId}
								class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
							>
								<option value="">Choose a member</option>
								{#each data.members as member (member.id)}
									<option value={member.id}>{member.name}</option>
								{/each}
							</select>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Field {form} name="provider">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Platform</Form.Label>
							<select
								{...props}
								bind:value={$fields.provider}
								class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
							>
								<option value="github">GitHub</option>
								<option value="gitlab">GitLab</option>
							</select>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Field {form} name="login">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Handle</Form.Label>
							<Input {...props} bind:value={$fields.login} placeholder="their-handle" />
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<div class="flex flex-wrap gap-2">
					<Button type="submit" disabled={$submitting}>
						{$submitting ? "Mapping…" : "Map them"}
					</Button>
				</div>
			</form>
		{/if}
	</div>
</div>
