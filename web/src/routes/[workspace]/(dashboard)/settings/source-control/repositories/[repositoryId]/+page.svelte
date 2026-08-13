<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";

	import { api } from "$lib/api";
	import { onDateAndTime } from "$lib/time";
	import * as Alert from "$lib/components/ui/alert";
	import { Button } from "$lib/components/ui/button";
	import * as Form from "$lib/components/ui/form";
	import { Input } from "$lib/components/ui/input";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Checkbox } from "$lib/components/ui/checkbox";
	import { Label } from "$lib/components/ui/label";
	import {
		connectionLabel,
		directionLabel,
		directionOrder,
		deliveryOutcomeLabel,
		detailOf,
		failureMessage,
		providerLabel,
		routeLabel,
		sourceControlConnectionPath,
		sourceControlFailure,
		sourceControlPath,
		type MirrorDirection,
		type SourceControlFailure,
		type SourceControlRepositoryView,
	} from "$lib/source-control/source-control";
	import { addRouteSchema } from "$lib/source-control/source-control-schema";
	import { sourceControlRepositoryPreviewStates } from "./preview";
	import type { PageData } from "./$types";

	let { data }: { data: PageData } = $props();

	const preview = $derived(
		import.meta.env.DEV
			? sourceControlRepositoryPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined,
	);

	let loaded = $state.raw<SourceControlRepositoryView | undefined>(undefined);
	let failure = $state.raw<SourceControlFailure | undefined>(undefined);
	let failureDetail = $state("");
	let removing = $state("");
	let disconnecting = $state(false);
	let confirmingRemove = $state(false);
	let savingDirection = $state(false);

	const view = $derived(loaded ?? preview?.view ?? data.view);
	const workspace = $derived(page.data.workspace);
	const timezone = $derived(page.data.session?.account?.timezone ?? "UTC");
	const shown = $derived(failure ?? preview?.failure);

	const teamName = $derived((id: string) => {
		const team = data.teams.find((one) => one.id === id);

		return team ? `${team.key} · ${team.name}` : "A team you cannot see";
	});

	function record(error: unknown) {
		const mapped = sourceControlFailure(error as never);

		failure = mapped;
		failureDetail = detailOf(error as never, mapped);
	}

	const routeForm = superForm(defaults(zod4(addRouteSchema)), {
		id: "add-source-control-route",
		SPA: true,
		validators: zod4Client(addRouteSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || view.kind !== "detail") return;

			failure = undefined;
			failureDetail = "";

			const { data: added, error } = await api.POST(
				"/workspaces/{workspaceId}/source-control/repositories/{repositoryId}/routes",
				{
					params: {
						path: { workspaceId: workspace.id, repositoryId: view.repository.id },
					},
					body: {
						teamId: entered.data.teamId,
						pathPrefix: entered.data.pathPrefix || undefined,
					},
				},
			);

			if (error) {
				const mapped = sourceControlFailure(error);
				const said = detailOf(error, mapped);

				if (mapped.kind === "already_routed") {
					setError(entered, "pathPrefix", said);
				} else {
					failure = mapped;
					failureDetail = said;
				}

				return;
			}

			if (!added || view.kind !== "detail") return;

			loaded = { ...view, routes: [added, ...view.routes] };
		},
	});

	const { form: routeFields, enhance: routeEnhance, submitting: routing } = routeForm;

	async function removeRoute(routeId: string) {
		if (view.kind !== "detail") return;

		removing = routeId;
		failure = undefined;
		failureDetail = "";

		const { error } = await api.DELETE(
			"/workspaces/{workspaceId}/source-control/routes/{routeId}",
			{ params: { path: { workspaceId: workspace.id, routeId } } },
		);

		removing = "";

		if (error) {
			record(error);

			return;
		}

		if (view.kind !== "detail") return;

		loaded = { ...view, routes: view.routes.filter((route) => route.id !== routeId) };
	}

	async function setDirection(direction: MirrorDirection) {
		if (view.kind !== "detail") return;

		savingDirection = true;
		failure = undefined;
		failureDetail = "";

		const { data: updated, error } = await api.PATCH(
			"/workspaces/{workspaceId}/source-control/repositories/{repositoryId}",
			{
				params: {
					path: { workspaceId: workspace.id, repositoryId: view.repository.id },
				},
				body: { syncDirection: direction },
			},
		);

		savingDirection = false;

		if (error) {
			record(error);

			return;
		}

		if (updated && view.kind === "detail") {
			loaded = { ...view, repository: updated };
		}
	}

	async function setPolling(disabled: boolean) {
		if (view.kind !== "detail") return;

		savingDirection = true;
		failure = undefined;
		failureDetail = "";

		const { data: updated, error } = await api.PATCH(
			"/workspaces/{workspaceId}/source-control/repositories/{repositoryId}",
			{
				params: {
					path: { workspaceId: workspace.id, repositoryId: view.repository.id },
				},
				body: { webhooksDisabled: disabled },
			},
		);

		savingDirection = false;

		if (error) {
			record(error);

			return;
		}

		if (updated && view.kind === "detail") {
			loaded = { ...view, repository: updated };
		}
	}

	async function disconnect() {
		if (view.kind !== "detail") return;

		disconnecting = true;
		failure = undefined;
		failureDetail = "";

		const { error } = await api.DELETE(
			"/workspaces/{workspaceId}/source-control/repositories/{repositoryId}",
			{
				params: { path: { workspaceId: workspace.id, repositoryId: view.repository.id } },
			},
		);

		disconnecting = false;

		if (error) {
			record(error);

			return;
		}

		await goto(sourceControlPath(workspace.slug));
	}
</script>

<svelte:head>
	<title>
		{view.kind === "detail" ? view.repository.fullName : "Repository"} · Source control ·
		{workspace.name} · Norn
	</title>
</svelte:head>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-col gap-1">
		<Eyebrow>
			<a href={sourceControlPath(workspace.slug)} class="hover:text-ink-900">Source control</a>
		</Eyebrow>
		<h1 class="text-lg font-medium tracking-snug text-ink-900">
			{view.kind === "detail" ? view.repository.fullName : "Repository"}
		</h1>
		{#if view.kind === "detail"}
			<p class="text-sm text-muted-foreground">
				Reached through
				<a
					href={sourceControlConnectionPath(workspace.slug, view.connection.id)}
					class="underline underline-offset-2"
				>
					{providerLabel(view.connection.provider)} · {connectionLabel(view.connection)}
				</a>
			</p>
		{/if}
	</div>

	{#if view.kind === "loading"}
		<p class="text-sm text-muted-foreground">Reading this repository…</p>
	{:else if view.kind === "not_found"}
		<Alert.Root>
			<Alert.Title>That repository is gone</Alert.Title>
			<Alert.Description>
				It may have been disconnected.
				<a href={sourceControlPath(workspace.slug)} class="underline">Back to source control</a>.
			</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "forbidden"}
		<Alert.Root>
			<Alert.Title>You cannot manage repositories</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "forbidden" })}</Alert.Description>
		</Alert.Root>
	{:else if view.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<Alert.Title>Source control could not be reached</Alert.Title>
			<Alert.Description>{failureMessage({ kind: "unavailable" })}</Alert.Description>
		</Alert.Root>
	{:else}
		{#if view.connection.authKind === "app"}
			<Alert.Root variant="muted">
				<Alert.Title>Deliveries arrive through the application</Alert.Title>
				<Alert.Description>
					The application has one webhook for everything it is installed on, so this repository
					needs none of its own. Changes are noticed at once.
				</Alert.Description>
			</Alert.Root>
		{:else if !view.repository.hookInstalled && !view.repository.webhooksDisabled}
			<Alert.Root>
				<TriangleAlert aria-hidden="true" />
				<Alert.Title>The webhook is not installed</Alert.Title>
				<Alert.Description>
					Norn could not install it — the token may not administer this repository. The sweep
					keeps trying, and until then changes are noticed on its schedule rather than at once.
				</Alert.Description>
			</Alert.Root>
		{/if}

		{#if shown}
			<Alert.Root variant="destructive">
				<Alert.Title>That did not work</Alert.Title>
				<Alert.Description>{failureDetail || failureMessage(shown)}</Alert.Description>
			</Alert.Root>
		{/if}

		<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Routes</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				A route narrows which team the changes under a path belong to. The longest matching path
				wins, and a route with no path is where everything else goes.
			</p>

			{#if view.routes.length === 0}
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					No route narrows this repository, so a change here can link an issue on any team. Add
					one only to restrict it — once a route exists, changes whose paths match none of them
					link nothing.
				</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each view.routes as route (route.id)}
						<li class="flex items-center justify-between gap-3">
							<div class="flex min-w-0 flex-col gap-0.5">
								<p class="truncate text-sm text-ink-900">{teamName(route.teamId)}</p>
								<p class="truncate font-mono text-xs text-muted-foreground">
									{routeLabel(route)}
								</p>
							</div>
							<Button
								variant="secondary"
								onclick={() => removeRoute(route.id)}
								disabled={removing === route.id}
							>
								{removing === route.id ? "Removing…" : "Remove"}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}

			<form method="POST" use:routeEnhance class="flex flex-col gap-4 border-t border-line-subtle pt-4">
				<Form.Field form={routeForm} name="teamId">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Team</Form.Label>
							<select
								{...props}
								bind:value={$routeFields.teamId}
								class="h-9 rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
							>
								<option value="">Choose a team</option>
								{#each data.teams as team (team.id)}
									<option value={team.id}>{team.key} · {team.name}</option>
								{/each}
							</select>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Field form={routeForm} name="pathPrefix">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Path</Form.Label>
							<Input
								{...props}
								bind:value={$routeFields.pathPrefix}
								placeholder="services/api — leave empty for everything else"
							/>
						{/snippet}
					</Form.Control>
					<Form.Description>
						Matching is by whole path segment, so “api” owns “api/main.go” and leaves
						“apiary/main.go” to whoever owns that.
					</Form.Description>
					<Form.FieldErrors />
				</Form.Field>

				<div class="flex flex-wrap gap-2">
					<Button type="submit" disabled={$routing}>
						{$routing ? "Adding…" : "Add a route"}
					</Button>
				</div>
			</form>
		</section>

		<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Which way work flows</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				A repository set one way is never touched the other. This is the promise that Norn
				leaves a forge alone when it was told to.
			</p>
			<div class="flex items-start gap-2.5">
				<Checkbox
					id="webhooks-disabled"
					checked={view.repository.webhooksDisabled ?? false}
					disabled={savingDirection}
					onCheckedChange={(checked) => setPolling(checked === true)}
				/>
				<div class="flex flex-col gap-0.5">
					<Label for="webhooks-disabled">This forge cannot reach Norn</Label>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Read this repository on a schedule instead of waiting to be told. Norn stops
						installing and repairing a webhook here, and changes are noticed at the interval
						below rather than at once.
					</p>
				</div>
			</div>

			<div class="flex flex-col gap-1">
				<Label for="sync-direction">Direction</Label>
				<select
					id="sync-direction"
					value={view.repository.syncDirection ?? "both"}
					onchange={(event) => setDirection(event.currentTarget.value as MirrorDirection)}
					disabled={savingDirection}
					class="h-9 max-w-md rounded-md border border-line-subtle bg-transparent px-3 text-sm text-ink-900"
				>
					{#each directionOrder as option (option)}
						<option value={option}>{directionLabel(option)}</option>
					{/each}
				</select>
			</div>
		</section>

		<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">What the platform sent</h2>

			{#if view.deliveries.length === 0}
				<p class="text-sm text-muted-foreground">
					Nothing has arrived yet. Push a branch naming an issue and it appears here.
				</p>
			{:else}
				<ul class="flex flex-col gap-3">
					{#each view.deliveries as delivery (delivery.id)}
						<li class="flex flex-col gap-0.5">
							<div class="flex flex-wrap items-baseline justify-between gap-2">
								<p class="text-sm text-ink-900">{delivery.event}</p>
								<p class="text-xs text-muted-foreground">
									{onDateAndTime(delivery.receivedAt, timezone)}
								</p>
							</div>
							<p class="text-sm text-muted-foreground">
								{deliveryOutcomeLabel(delivery)}{#if delivery.detail}
									 — {delivery.detail}{/if}
							</p>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<section class="flex flex-col gap-3 rounded-lg border border-destructive/40 p-4">
			<h2 class="text-md font-medium tracking-snug text-ink-900">Disconnect this repository</h2>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				{view.connection.authKind === "app"
					? "Its routes go with it."
					: "Its webhook is removed and its routes go with it."} Links and mirrors already on issues
				stay exactly as readable. The credential itself is untouched.
			</p>

			{#if confirmingRemove}
				<div class="flex flex-wrap gap-2">
					<Button variant="destructive" onclick={disconnect} disabled={disconnecting}>
						{disconnecting ? "Disconnecting…" : "Yes, disconnect"}
					</Button>
					<Button
						variant="secondary"
						onclick={() => (confirmingRemove = false)}
						disabled={disconnecting}
					>
						Keep it
					</Button>
				</div>
			{:else}
				<Button variant="secondary" onclick={() => (confirmingRemove = true)} class="self-start">
					Disconnect
				</Button>
			{/if}
		</section>
	{/if}
</div>
