<script lang="ts">
	import { goto, invalidate } from "$app/navigation";
	import { keys } from "$lib/api/keys";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Folder from "@lucide/svelte/icons/folder";
	import Target from "@lucide/svelte/icons/target";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Empty from "$lib/components/ui/empty/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { listCursor } from "$lib/shortcuts/list-cursor.svelte";
	import ShortcutBar from "$lib/shortcuts/shortcut-bar.svelte";
	import {
		healthLabel,
		projectFailureMessage,
		projectPath,
		projectsPath,
		projectStates,
		readProjectFailure,
		stateLabel,
		type Project,
		type ProjectFailure,
	} from "$lib/projects/projects";
	import { onCalendarDate } from "$lib/time";
	import { slugFromName } from "$lib/workspace/create-workspace-schema";
	import { projectsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const slug = $derived(data.workspace.slug);
	const preview = $derived(
		import.meta.env.DEV
			? projectsPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
	const listing = $derived(preview?.listing ?? data.listing);
	const showingArchived = $derived(page.url.searchParams.get("archived") === "1");
	const creating = $derived(page.url.searchParams.has("new"));
	const teamQuery = $derived(data.team ? `teamId=${data.team.id}&` : "");

	const grouped = $derived.by(() => {
		if (listing.kind !== "ready") return [];

		return projectStates
			.map((state) => ({
				state,
				projects: listing.projects.filter((project) => project.state === state),
			}))
			.filter((group) => group.projects.length > 0);
	});

	const rows = $derived(grouped.flatMap((group) => group.projects));

	const cursor = listCursor(() => ({
		rows,
		open: (project) => void goto(projectPath(slug, project)),
	}));

	let name = $state("");
	let address = $state("");
	let addressEdited = $state(false);
	let working = $state(false);
	let failure = $state<ProjectFailure | null>(null);

	const derivedAddress = $derived(addressEdited ? address : slugFromName(name));

	function dismiss() {
		const next = new URL(page.url);
		next.searchParams.delete("new");
		goto(next, { replaceState: true, noScroll: true, keepFocus: true });
	}

	async function create() {
		if (!name.trim() || !derivedAddress) return;

		working = true;
		failure = null;

		try {
			const { data: created, error } = await api.POST("/workspaces/{workspaceId}/projects", {
				params: { path: { workspaceId: data.workspace.id } },
				body: {
					name: name.trim(),
					slug: derivedAddress,
					...(data.team ? { teamIds: [data.team.id] } : {}),
				},
			});

			if (error) {
				failure = readProjectFailure(error);

				return;
			}

			if (created) {
				await goto(projectPath(slug, created));

				return;
			}

			dismiss();
			await invalidate(keys.projects(data.workspace.id));
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	function targetLabel(project: Project): string {
		return project.targetOn ? `Target ${onCalendarDate(project.targetOn)}` : "No target date";
	}
</script>

<svelte:head>
	<title>{data.team ? `${data.team.name} projects` : "Projects"} · {data.workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Target class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="min-w-0 truncate text-md font-medium tracking-snug text-ink-900">
				{data.team ? data.team.name : "Projects"}
			</h1>
			{#if data.team}
				<a
					href={projectsPath(slug)}
					class="shrink-0 rounded-sm border border-line-default px-1.5 py-0.5 text-sm whitespace-nowrap text-muted-foreground hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
				>
					Every project
				</a>
			{/if}
			<div class="ml-auto flex items-center gap-2">
				<Button variant="ghost" size="sm" href="?{teamQuery}archived={showingArchived ? '0' : '1'}">
					{showingArchived ? "Hide archived" : "Show archived"}
				</Button>
				<Button variant="secondary" size="sm" href="?{teamQuery}new">New project</Button>
			</div>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not go through</Alert.Title>
					<Alert.Description>{projectFailureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if creating}
				<section class="flex flex-col gap-3 rounded-lg border border-line-strong bg-paper-1 p-3">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Start a project</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							A body of work with a goal and an end.
							{#if data.team}
								It starts under {data.team.name} and can still draw issues from any team you can
								see.
							{:else}
								It can draw issues from any team you can see.
							{/if}
						</p>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="project-name" class="text-sm font-medium text-ink-900">Name</label>
						<Input
							id="project-name"
							placeholder="Checkout rebuild"
							disabled={working}
							bind:value={name}
						/>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="project-address" class="text-sm font-medium text-ink-900">Address</label>
						<Input
							id="project-address"
							disabled={working}
							value={derivedAddress}
							oninput={(event) => {
								addressEdited = true;
								address = event.currentTarget.value;
							}}
						/>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							The project will live at /{slug}/projects/{derivedAddress || "…"}. It does not change
							when the project is renamed.
						</p>
					</div>

					<div class="flex gap-2">
						<Button disabled={working || !name.trim()} onclick={create}>
							{working ? "Starting" : "Start project"}
						</Button>
						<Button variant="secondary" disabled={working} onclick={dismiss}>Cancel</Button>
					</div>
				</section>
			{/if}

			{#if listing.kind === "loading"}
				<ul class="flex flex-col gap-px" aria-busy="true">
					{#each [0, 1, 2] as row (row)}
						<li class="h-14 animate-breathe rounded-md bg-paper-2"></li>
					{/each}
				</ul>
			{:else if listing.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load your projects</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else if listing.kind === "empty"}
				<Empty.Root>
					<Empty.Media variant="icon"><Folder aria-hidden="true" /></Empty.Media>
					<Empty.Header>
						<Empty.Title>No projects yet</Empty.Title>
						<Empty.Description>
							A project holds work with a goal and an end date, across teams. Cycles stay for the
							week-to-week.
						</Empty.Description>
					</Empty.Header>
					<Empty.Content>
						<Button size="sm" href="?{teamQuery}new">New project</Button>
					</Empty.Content>
				</Empty.Root>
			{:else}
				{#each grouped as group (group.state)}
					<section class="flex flex-col gap-2">
						<Eyebrow class="text-ink-600">{stateLabel(group.state)}</Eyebrow>
						<ul class="flex flex-col rounded-lg border border-line-subtle">
							{#each group.projects as project (project.id)}
								<li
									class="cursor-row border-b border-line-subtle last:border-b-0"
									{...cursor.props(project)}
								>
									<a
										href={projectPath(slug, project)}
										class="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2.5 motion-control hover:bg-paper-2 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
									>
										<span class="min-w-0 flex-[1_1_140px] truncate text-md text-ink-900">
											{project.name}
										</span>
										{#if project.archived}
											<Tag name="Archived" />
										{/if}
										{#if project.health}
											<Tag
												name={healthLabel(project.health)}
												color={project.health === "on_track" ? "cyan" : "orchid"}
											/>
										{/if}
										<span class="shrink-0 text-sm text-muted-foreground">
											{project.leadName || "No lead"}
										</span>
										<span class="shrink-0 font-mono text-xs text-muted-foreground tabular-nums">
											{targetLabel(project)}
										</span>
									</a>
								</li>
							{/each}
						</ul>
					</section>
				{/each}
			{/if}
		</div>
	</div>

	<ShortcutBar ids={["cursor-down", "cursor-open", "issue-new", "search", "help"]} />
</div>
