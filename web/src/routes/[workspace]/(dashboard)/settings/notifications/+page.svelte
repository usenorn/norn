<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import Bell from "@lucide/svelte/icons/bell";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import MailWarning from "@lucide/svelte/icons/mail-warning";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import PreferenceGrid from "$lib/notifications/preference-grid.svelte";
	import type { NotificationPreferences } from "$lib/notifications/notifications";
	import { workspacePath } from "$lib/workspace/navigation";
	import { notificationSettingsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";
	import { showToast } from "$lib/toast/toasts";

	let { data }: PageProps = $props();


	const preview = $derived(
		import.meta.env.DEV
			? notificationSettingsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let draft = $state.raw<NotificationPreferences | null>(null);
	let saving = $state(false);
	let failed = $state(false);

	const panel = $derived(preview?.panel ?? data.panel);
	const slug = $derived(data.workspace.slug);
	const stored = $derived(panel.kind === "ready" ? panel.settings.preferences : null);
	const preferences = $derived(draft ?? stored);
	const emailEnabled = $derived(panel.kind === "ready" ? panel.settings.emailEnabled : false);
	const dirty = $derived(draft !== null && JSON.stringify(draft) !== JSON.stringify(stored));

	async function save() {
		if (!draft) return;

		saving = true;
		failed = false;

		try {
			const { error } = await api.PUT("/workspaces/{workspaceId}/notification-settings", {
				params: { path: { workspaceId: data.workspace.id } },
				body: draft,
			});

			if (error) {
				failed = true;

				return;
			}

			showToast("Your notification settings are saved.");
			draft = null;
			await invalidate(keys.page(page.route.id));
		} catch {
			failed = true;
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head><title>Notifications · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Bell class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">
				Notifications
			</h1>
			<span class="text-sm text-muted-foreground">What reaches you, and how</span>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>

			{#if failed}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not save</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if panel.kind === "loading"}
				<div class="h-60 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if panel.kind === "unavailable"}
				<p class="text-md leading-normal text-muted-foreground">
					We could not load your notification settings. Nothing changed &mdash; wait a moment and
					try again.
				</p>
			{:else if preferences}
				{#if !emailEnabled}
					<Alert.Root>
						<MailWarning aria-hidden="true" />
						<Alert.Title>Email is not configured on this instance</Alert.Title>
						<Alert.Description>
							Your inbox works as normal. Nothing will be sent by email until an administrator
							configures a mail server.
						</Alert.Description>
					</Alert.Root>
				{/if}

				<PreferenceGrid
					{preferences}
					{emailEnabled}
					disabled={saving}
					idPrefix="workspace"
					onchange={(next) => (draft = next)}
				/>

				<div class="flex flex-wrap items-center gap-2">
					<Button size="sm" disabled={!dirty || saving} onclick={save}>
						{saving ? "Saving…" : "Save"}
					</Button>
					<Button
						size="sm"
						variant="ghost"
						disabled={!dirty || saving}
						onclick={() => (draft = null)}
					>
						Discard
					</Button>
					<p class="text-sm text-muted-foreground">
						A team can differ from this &mdash; set that on the team's own settings.
					</p>
				</div>

				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Email arrives as one digest rather than one message per change, and never repeats
					something you already read in your <a
						href={workspacePath(slug, "/inbox")}
						class="underline underline-offset-2 hover:text-ink-900">inbox</a
					>.
				</p>
			{/if}
		</div>
	</div>
</div>
