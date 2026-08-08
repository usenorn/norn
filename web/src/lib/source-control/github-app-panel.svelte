<script lang="ts">
	import { tick } from "svelte";
	import { goto } from "$app/navigation";
	import CircleCheck from "@lucide/svelte/icons/circle-check";

	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
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
	}: {
		workspaceId: string;
		workspaceSlug: string;
		application: SourceControlAppState;
		notice?: SourceControlAppNotice;
	} = $props();

	let manifestAction = $state("");
	let manifestBody = $state("");
	let manifestForm = $state.raw<HTMLFormElement | undefined>(undefined);

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
			{ params: { path: { workspaceId } }, body: {} },
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

	{#if application.kind === "choosing"}
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
			Norn acts as an installed application rather than as one person. Install it on the
			repositories you want watched, then sign in to choose the installation. Nobody's personal
			token is involved, and the connection survives them leaving.
		</p>
		<div class="flex flex-col gap-2 sm:flex-row">
			<Button disabled={working} onclick={signIn}>
				{working ? "Opening GitHub…" : "Connect GitHub"}
			</Button>
			{#if application.installUrl}
				<Button variant="secondary" href={application.installUrl} rel="external">
					Install it on repositories
				</Button>
			{/if}
		</div>
	{:else if application.kind === "unregistered" && application.canRegister}
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			This instance holds no GitHub application yet. Registering one takes you to GitHub to
			confirm; it hands back the keys, and nothing is copied by hand.
		</p>
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
