<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import Bell from "@lucide/svelte/icons/bell";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Skeleton } from "$lib/components/ui/skeleton/index.js";
	import SettingsPage from "$lib/settings/settings-page.svelte";
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

<SettingsPage
	title="Notifications"
	description="Choose which activity from this workspace reaches you, and how."
	Icon={Bell}
	meta="per workspace"
	width="compact"
>

			{#if failed}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not save</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if panel.kind === "loading"}
				<div class="flex flex-col gap-3" aria-busy="true" aria-label="Loading notification settings">
					<Skeleton class="h-7 w-48" />
					<Skeleton class="h-44 w-full" />
					<Skeleton class="h-8 w-28" />
				</div>
			{:else if panel.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>Could not load your notification settings</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else if preferences}
				<PreferenceGrid
					{preferences}
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
</SettingsPage>
