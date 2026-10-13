<script lang="ts">
	import Copy from "@lucide/svelte/icons/copy";
	import Eye from "@lucide/svelte/icons/eye";
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Send from "@lucide/svelte/icons/send";
	import Pill, { type PillTone } from "$lib/components/norn/pill.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { copyText } from "$lib/clipboard";
	import SettingsSectionHeader from "$lib/settings/settings-section-header.svelte";
	import { onDateAndTime } from "$lib/time";
	import { memberName, type Membership } from "$lib/workspace/members";
	import {
		publicationLabels,
		roadmapPreviewPath,
		type RoadmapActivity,
		type RoadmapPublicationState,
	} from "./admin";

	let {
		publication,
		activity,
		selfHosted,
		members,
		timezone,
		workspace,
		onreview,
		onunpublish,
	}: {
		publication: RoadmapPublicationState;
		activity: RoadmapActivity;
		selfHosted: boolean;
		members: Membership[];
		timezone: string;
		workspace: string;
		onreview: () => void;
		onunpublish: () => void;
	} = $props();

	const tones: Record<RoadmapPublicationState["kind"], PillTone> = {
		unpublished: "muted",
		published: "success",
		unpublished_changes: "warning",
	};

	const busy = $derived(activity.kind !== "idle");
	const live = $derived(publication.kind === "unpublished" ? undefined : publication);
	const url = $derived(publication.url);
	const publisher = $derived(
		live?.publication.publishedByAccountId
			? members.find((member) => member.accountId === live.publication.publishedByAccountId)
			: undefined
	);

	const publishedLine = $derived.by(() => {
		if (!live) return "";

		const when = onDateAndTime(live.publication.publishedAt, timezone);

		return publisher ? `Published ${when} by ${memberName(publisher)}.` : `Published ${when}.`;
	});
</script>

<section
	class="flex flex-col gap-3 rounded-lg border border-line-subtle p-4"
	aria-labelledby="roadmap-publication"
>
	<div class="flex flex-col gap-1">
		<SettingsSectionHeader id="roadmap-publication" title="Publication">
			{#snippet badge()}
				<Pill tone={tones[publication.kind]}>{publicationLabels[publication.kind]}</Pill>
			{/snippet}
			{#if live}
				{publishedLine}
				{#if publication.kind === "unpublished_changes"}
					The draft differs from what readers see. Review it and publish again to update the public page.
				{:else}
					Readers see exactly this draft.
				{/if}
			{:else}
				Nothing is public. Choose what to share, preview it, then review and publish.
			{/if}
		</SettingsSectionHeader>

		{#if live && live.withheld > 0}
			<p class="text-sm leading-normal text-warning text-pretty">
				{live.withheld === 1 ? "1 entry is" : `${live.withheld} entries are`} hidden publicly because the project
				or issue behind it changed. Publish again to show the current copy, or remove the entry.
			</p>
		{/if}

		{#if url}
			<p class="font-mono text-xs break-all text-ink-900">{url}</p>
		{:else if selfHosted}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				This instance has no roadmap domain, so there is no link to share. Publishing still works; an operator
				sets NORN_ROADMAPS_DOMAIN to give published roadmaps an address.
			</p>
		{:else}
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				This roadmap has no public address yet, so there is no link to share. Publishing still works; readers can
				reach it once the address exists.
			</p>
		{/if}
	</div>

	<div class="flex flex-wrap items-center gap-2">
		<Button size="sm" onclick={onreview} disabled={busy}>
			<Send aria-hidden="true" />
			Review and publish
		</Button>
		<Button size="sm" variant="secondary" href={roadmapPreviewPath(workspace)} disabled={busy}>
			<Eye aria-hidden="true" />
			Preview draft
		</Button>
		{#if live && url}
			<Button
				size="sm"
				variant="secondary"
				disabled={busy}
				onclick={() => copyText(url, "Public link copied")}
			>
				<Copy aria-hidden="true" />
				Copy public link
			</Button>
			<Button size="sm" variant="ghost" href={url} target="_blank" rel="noopener noreferrer" disabled={busy}>
				<ExternalLink aria-hidden="true" />
				Open
			</Button>
		{/if}
		{#if live}
			<Button size="sm" variant="ghost" class="sm:ml-auto" onclick={onunpublish} disabled={busy}>
				{activity.kind === "unpublishing" ? "Unpublishing" : "Unpublish"}
			</Button>
		{/if}
	</div>
</section>
