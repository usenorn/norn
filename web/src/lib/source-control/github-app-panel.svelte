<script lang="ts">
	import { tick } from "svelte";
	import { goto } from "$app/navigation";
	import CircleCheck from "@lucide/svelte/icons/circle-check";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import {
		appNoticeMessage,
		detailOf,
		failureMessage,
		sourceControlFailure,
		sourceControlPath,
		type SourceControlAppNotice,
		type SourceControlAppState,
		type SourceControlFailure,
	} from "./source-control";

	let {
		workspaceId,
		workspaceSlug,
		application,
		notice,
		installedOn = null,
		connectedTo = null,
		repositoryCount = 0,
		connectHref = "",
	}: {
		workspaceId: string;
		workspaceSlug: string;
		application: SourceControlAppState;
		/**
		 * The account this workspace's own connection installs through. One application serves
		 * every workspace on the instance, so where else it is installed is not this screen's
		 * to know or to name.
		 */
		installedOn?: string | null;
		connectedTo?: string | null;
		/** How many repositories the workspace watches. Nothing works until this is above zero. */
		repositoryCount?: number;
		connectHref?: string;
		notice?: SourceControlAppNotice;
	} = $props();

	let manifestAction = $state("");
	let manifestBody = $state("");
	let manifestForm = $state.raw<HTMLFormElement | undefined>(undefined);

	let onOwnNetwork = $state(false);
	let authority = $state("");
	let working = $state(false);
	let chosen = $state("");
	let failure = $state.raw<SourceControlFailure | undefined>(undefined);
	let failureDetail = $state("");

	function refused(problem: Parameters<typeof sourceControlFailure>[0]) {
		const mapped = sourceControlFailure(problem);

		failure = mapped;
		failureDetail = detailOf(problem, mapped);
	}

	async function register() {
		working = true;
		failure = undefined;

		const { data, error } = await api.POST(
			"/workspaces/{workspaceId}/source-control/application/registration",
			{
				params: { path: { workspaceId } },
				body: {
					allowPrivateAddress: onOwnNetwork || undefined,
					caCertificate: authority.trim() || undefined,
				},
			},
		);

		working = false;

		if (error) {
			refused(error);

			return;
		}

		if (!data) return;

		manifestAction = `${data.target}?state=${encodeURIComponent(data.state)}`;
		manifestBody = data.manifest;

		await tick();
		manifestForm?.submit();
	}

	async function signIn() {
		working = true;
		failure = undefined;

		const { data, error } = await api.POST(
			"/workspaces/{workspaceId}/source-control/application/authorization",
			{ params: { path: { workspaceId } } },
		);

		working = false;

		if (error) {
			refused(error);

			return;
		}

		if (data) window.location.href = data.url;
	}

	async function use(handle: string, installationId: string) {
		working = true;
		chosen = installationId;
		failure = undefined;

		const { error } = await api.POST("/workspaces/{workspaceId}/source-control/connections", {
			params: { path: { workspaceId } },
			body: { provider: "github", installationHandle: handle, installationId },
		});

		working = false;
		chosen = "";

		if (error) {
			refused(error);

			return;
		}

		await goto(sourceControlPath(workspaceSlug), { invalidateAll: true });
	}
</script>

<section class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4">
	<h2 class="text-md font-medium tracking-snug text-ink-900">GitHub</h2>

	{#if notice?.kind === "registered"}
		<Alert.Root variant="success">
			<CircleCheck aria-hidden="true" />
			<Alert.Title>The application is registered</Alert.Title>
			<Alert.Description>{appNoticeMessage(notice)}</Alert.Description>
		</Alert.Root>
	{:else if notice}
		<Alert.Root variant="destructive">
			<Alert.Title>That did not finish</Alert.Title>
			<Alert.Description>{appNoticeMessage(notice)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if failure}
		<Alert.Root variant="destructive">
			<Alert.Title>That did not work</Alert.Title>
			<Alert.Description>{failureDetail || failureMessage(failure)}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if application.kind === "choosing" && application.installations.length === 0}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			You are signed in to GitHub, but the application is installed on no account you
			administer, so there is nothing to choose. Install it, then connect again.
		</p>

		{#if application.installUrl}
			<Button href={application.installUrl} rel="external" class="w-max">
				Install on GitHub
			</Button>
		{/if}
	{:else if application.kind === "choosing"}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Choose which installation Norn acts through. Only the repositories you granted it are
			reachable, and you can change that on GitHub at any time.
		</p>

		<ul class="flex flex-col gap-2">
			{#each application.installations as installation (installation.externalId)}
				<li
					class="flex flex-col gap-2 rounded-md border border-line-subtle p-3 sm:flex-row sm:items-center sm:justify-between"
				>
					<div class="flex min-w-0 flex-col gap-0.5">
						<p class="truncate text-sm font-medium text-ink-900">
							{installation.accountLogin}
						</p>
						{#if installation.accountKind}
							<p class="text-sm text-muted-foreground">{installation.accountKind}</p>
						{/if}
					</div>
					<Button
						variant="secondary"
						disabled={working}
						onclick={() => use(application.handle, installation.externalId)}
					>
						{chosen === installation.externalId ? "Connecting…" : "Use this one"}
					</Button>
				</li>
			{/each}
		</ul>
	{:else if application.kind === "registered"}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Norn acts as an installed application rather than as one person. Nobody's personal token
			is involved, and the connection survives them leaving.
		</p>

		<ol class="flex flex-col gap-4">
			<li class="flex min-w-0 gap-3">
				<span
					class="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border font-mono text-2xs {installedOn
						? 'border-success/40 text-success'
						: 'border-line-strong text-ink-900'}"
					aria-hidden="true"
				>
					{installedOn ? "✓" : "1"}
				</span>
				<div class="flex min-w-0 flex-1 flex-col gap-1.5">
					<p class="text-sm leading-normal text-ink-900 text-pretty">
						{#if installedOn}
							Installed on {installedOn}.
						{:else}
							Install it on the organisation and repositories you want watched.
						{/if}
					</p>
					{#if application.installUrl}
						<Button
							variant={installedOn ? "secondary" : "default"}
							size="sm"
							class="w-max"
							href={application.installUrl}
							rel="external"
						>
							{installedOn ? "Change permissions" : "Install on GitHub"}
						</Button>
					{/if}
				</div>
			</li>

			<li class="flex min-w-0 gap-3">
				<span
					class="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border font-mono text-2xs {connectedTo
						? 'border-success/40 text-success'
						: 'border-line-strong text-ink-900'}"
					aria-hidden="true"
				>
					{connectedTo ? "✓" : "2"}
				</span>
				<div class="flex min-w-0 flex-1 flex-col gap-1.5">
					<p class="text-sm leading-normal text-ink-900 text-pretty">
						{#if connectedTo}
							Connected as {connectedTo}.
						{:else}
							Sign in to GitHub and choose which installation Norn acts through.
						{/if}
					</p>
					{#if !connectedTo}
						<Button size="sm" class="w-max" disabled={working} onclick={signIn}>
							{working ? "Opening GitHub…" : "Connect GitHub"}
						</Button>
					{/if}
				</div>
			</li>

			<li class="flex min-w-0 gap-3">
				<span
					class="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border font-mono text-2xs {repositoryCount >
					0
						? 'border-success/40 text-success'
						: connectedTo
							? 'border-line-strong text-ink-900'
							: 'border-line-subtle text-muted-foreground'}"
					aria-hidden="true"
				>
					{repositoryCount > 0 ? "✓" : "3"}
				</span>
				<div class="flex min-w-0 flex-1 flex-col gap-1.5">
					<p
						class="text-sm leading-normal text-pretty {connectedTo
							? 'text-ink-900'
							: 'text-muted-foreground'}"
					>
						{#if repositoryCount === 1}
							Watching 1 repository.
						{:else if repositoryCount > 1}
							Watching {repositoryCount} repositories.
						{:else if connectedTo}
							Choose the repositories Norn watches. Until one is connected, everything GitHub
							sends is discarded.
						{:else}
							Then choose the repositories Norn watches.
						{/if}
					</p>
					{#if connectedTo && repositoryCount === 0 && connectHref}
						<Button size="sm" class="w-max" href={connectHref}>Connect a repository</Button>
					{/if}
				</div>
			</li>
		</ol>
	{:else if application.kind === "unregistered" && application.canRegister}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			This instance holds no GitHub application yet. Registering one takes you to GitHub to
			confirm; it hands back the keys, and nothing is copied by hand.
		</p>
		<div class="flex items-start gap-2.5">
			<Checkbox
				id="application-on-own-network"
				checked={onOwnNetwork}
				disabled={working}
				onCheckedChange={(checked) => (onOwnNetwork = checked === true)}
			/>
			<div class="flex flex-col gap-0.5">
				<Label for="application-on-own-network">This instance is on our own network</Label>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Grant this application the exception it needs to reach an enterprise instance on a
					private address. Leave it off for github.com.
				</p>
			</div>
		</div>

		{#if onOwnNetwork}
			<div class="flex flex-col gap-1.5">
				<Label for="application-authority">Certificate authority</Label>
				<Textarea
					id="application-authority"
					bind:value={authority}
					disabled={working}
					rows={4}
					placeholder="-----BEGIN CERTIFICATE-----"
				/>
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Only needed when the instance presents a certificate no public authority signed.
				</p>
			</div>
		{/if}

		<Button disabled={working} onclick={register} class="self-start">
			{working ? "Opening GitHub…" : "Register the application"}
		</Button>
	{:else}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			This instance cannot act as a GitHub application. Hold a personal access token below
			instead.
		</p>
	{/if}
</section>

{#if manifestAction}
	<form bind:this={manifestForm} method="POST" action={manifestAction} class="hidden">
		<input type="hidden" name="manifest" value={manifestBody} />
	</form>
{/if}
