<script lang="ts">
	import { goto, invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Archive from "@lucide/svelte/icons/archive";
	import Link2 from "@lucide/svelte/icons/link-2";
	import ArchiveRestore from "@lucide/svelte/icons/archive-restore";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import BellOff from "@lucide/svelte/icons/bell-off";
	import BellRing from "@lucide/svelte/icons/bell-ring";
	import List from "@lucide/svelte/icons/list";
	import Tags from "@lucide/svelte/icons/tags";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { cycleWindow, dueLabel, onDate, onDateAndTime, overdue } from "$lib/time";
	import Markdown from "$lib/issues/markdown.svelte";
	import {
		issueFailureMessage,
		priorities,
		priorityLabel,
		readIssueFailure,
		statusLabel,
		type IssueFailure,
		type IssuePriority,
		type IssueRelationKind,
	} from "$lib/issues/issues";
	import { issueEditSchema } from "$lib/issues/issue-schema";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Layers from "@lucide/svelte/icons/layers";
	import Target from "@lucide/svelte/icons/target";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import IssueParent from "$lib/issues/issue-parent.svelte";
	import IssueChildren from "$lib/issues/issue-children.svelte";
	import IssueRelations from "$lib/issues/issue-relations.svelte";
	import CommentThreadView from "$lib/comments/comment-thread.svelte";
	import ActivityFeedView from "$lib/activity/activity-feed.svelte";
	import type { ActivityFeed } from "$lib/activity/activity";
	import AttachmentList from "$lib/attachments/attachment-list.svelte";
	import AttachmentPicker from "$lib/attachments/attachment-picker.svelte";
	import UploadList from "$lib/attachments/upload-list.svelte";
	import {
		attachmentMarkdown,
		attachmentFailureMessage,
		readAttachmentFailure,
		type AttachmentFailure,
		type AttachmentPanel,
	} from "$lib/attachments/attachments";
	import { newTask, settled, upload, type UploadTask } from "$lib/attachments/upload";
	import {
		readCommentFailure,
		type CommentFailure,
		type CommentMention,
		type CommentReaction,
		type CommentThread,
		type MentionTarget,
	} from "$lib/comments/comments";
	import {
		conflictFailure,
		labelFailureMessage,
		sectioned,
		selectable,
		toggled,
		type Label,
		type LabelFailure,
	} from "$lib/labels/labels";
	import { workspacePath } from "$lib/workspace/navigation";
	import {
		activityPreviewStates,
		attachmentPreviewStates,
		commentPreviewStates,
		issueDetailPreviewStates,
	} from "./preview";
	import type { IssueDetail } from "./+page";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? issueDetailPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);
	const attachmentPreview = $derived(
		import.meta.env.DEV
			? attachmentPreviewStates[page.url.searchParams.get("attachments") ?? ""]
			: undefined
	);
	const activityPreview = $derived(
		import.meta.env.DEV
			? activityPreviewStates[page.url.searchParams.get("activity") ?? ""]
			: undefined
	);
	const commentPreview = $derived(
		import.meta.env.DEV
			? commentPreviewStates[page.url.searchParams.get("comments") ?? ""]
			: undefined
	);

	let applied = $state<Label[] | null>(null);
	let labelFailure = $state<LabelFailure | null>(null);
	let failure = $state<IssueFailure | null>(null);
	let editing = $state(false);
	let pendingTeamId = $state("");
	let commentFailure = $state<CommentFailure | null>(null);
	let unreachable = $state.raw<CommentMention[]>([]);
	let loadedComments = $state.raw<CommentThread | null>(null);
	let loadedActivity = $state.raw<ActivityFeed | null>(null);
	let commentUploads = $state.raw<UploadTask[]>([]);
	let bodyUploads = $state.raw<UploadTask[]>([]);
	let attachmentFailure = $state<AttachmentFailure | null>(null);
	let uploadSequence = 0;

	const aborts = new Map<string, () => void>();
	const sources = new Map<string, File>();
	let pendingStateId = $state("");
	let followWorking = $state(false);

	async function toggleFollow() {
		if (!issue) return;

		followWorking = true;

		try {
			const { error } = await api.PUT("/workspaces/{workspaceId}/issues/{issueId}/follow", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { state: following ? "muted" : "following" },
			});

			if (error) return;

			announcement = following
				? `You will not be notified about ${issue.reference} unless someone names you.`
				: `You are following ${issue.reference}.`;

			await invalidateAll();
		} finally {
			followWorking = false;
		}
	}

	const detail = $derived<IssueDetail>(preview?.detail ?? data.detail);
	const ready = $derived(detail.kind === "ready" ? detail : null);
	const issue = $derived(ready?.issue ?? null);
	const following = $derived(ready?.follow === "following");
	const labels = $derived(applied ?? issue?.labels ?? []);
	const slug = $derived(page.params.workspace ?? "");
	const at = $derived((path: string) => workspacePath(slug, path));

	const available = $derived(
		issue ? selectable(ready?.labels ?? [], issue.teamId) : ([] as Label[])
	);
	const sections = $derived(sectioned(available, ready?.groups ?? []));

	const teams = $derived(data.teams ?? []);
	const assigneeName = $derived(
		ready?.members.find((member) => member.accountId === issue?.assigneeAccountId)?.displayName ?? ""
	);
	const attachments = $derived<AttachmentPanel>(
		attachmentPreview?.panel ??
			(ready ? ready.attachments : ({ kind: "loading" } as AttachmentPanel))
	);
	const activity = $derived<ActivityFeed>(
		activityPreview?.feed ?? loadedActivity ?? ready?.activity ?? { kind: "loading" }
	);
	const shownCommentUploads = $derived(attachmentPreview?.uploads ?? commentUploads);

	const thread = $derived<CommentThread>(
		commentPreview?.thread ?? loadedComments ?? ready?.comments ?? { kind: "loading" }
	);

	const form = superForm(defaults(zod4(issueEditSchema)), {
		id: "issue-edit",
		SPA: true,
		validators: zod4Client(issueEditSchema),
		resetForm: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid || !issue) return;

			const clear: string[] = [];
			const body: Record<string, unknown> = {};

			if (pending.data.title !== issue.title) body.title = pending.data.title;
			if (pending.data.description !== issue.description) body.description = pending.data.description;

			if (pending.data.estimate === "") {
				if (issue.estimate) clear.push("estimate");
			} else if (Number(pending.data.estimate) !== issue.estimate) {
				body.estimate = Number(pending.data.estimate);
			}

			if (pending.data.dueOn === "") {
				if (issue.dueOn) clear.push("dueOn");
			} else if (pending.data.dueOn !== issue.dueOn) {
				body.dueOn = pending.data.dueOn;
			}

			if (clear.length > 0) body.clear = clear;

			if (Object.keys(body).length === 0) {
				editing = false;

				return;
			}

			if (await patch(body)) {
				editing = false;
				announcement = "Saved.";

				return;
			}

			if (failure?.kind === "invalid") {
				for (const field of failure.fields) {
					if (field === "title") setError(pending, "title", "Give the issue a title.");
					if (field === "description") setError(pending, "description", "That description cannot be stored.");
					if (field === "estimate") setError(pending, "estimate", "Use a whole number of points.");
					if (field === "dueOn") setError(pending, "dueOn", "Use a date like 2026-09-01.");
				}
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		if (!issue || editing) return;

		const seed = {
			title: issue.title,
			description: issue.description,
			priority: issue.priority,
			assigneeId: issue.assigneeAccountId ?? "",
			estimate: issue.estimate ? String(issue.estimate) : "",
			dueOn: issue.dueOn ?? "",
		};

		formData.update((current) => ({ ...current, ...seed }), { taint: false });
	});
	let working = $state(false);
	let announcement = $state("");

	function readFailure(error: unknown): LabelFailure {
		if (error && typeof error === "object" && "code" in error) {
			const problem = error as { code: string; issues?: number; conflicts?: string[] };
			const conflict = conflictFailure(problem.code, problem.issues, problem.conflicts);

			if (conflict) return conflict;
		}

		if (error && typeof error === "object" && "status" in error) {
			if ((error as { status: number }).status === 403) return { kind: "forbidden" };
		}

		return { kind: "unavailable" };
	}

	async function submit(labelIds: string[]) {
		if (!issue) return;

		working = true;
		labelFailure = null;
		failure = null;

		try {
			const { data: next, error } = await api.PUT(
				"/workspaces/{workspaceId}/issues/{issueId}/labels",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: { expectedVersion: issue.version, labelIds },
				}
			);

			if (error) {
				labelFailure = readFailure(error);
				applied = null;
				await invalidateAll();

				return;
			}

			applied = next ?? [];
			announcement = `This issue now carries ${applied.length} ${applied.length === 1 ? "label" : "labels"}.`;
			await invalidateAll();
		} catch {
			labelFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function patch(body: Record<string, unknown>): Promise<boolean> {
		if (!issue) return false;

		working = true;
		failure = null;
		labelFailure = null;

		try {
			const { error } = await api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { expectedVersion: issue.version, ...body },
			});

			if (error) {
				failure = readIssueFailure(error);
				await invalidateAll();

				return false;
			}

			await invalidateAll();

			return true;
		} catch {
			failure = { kind: "unavailable" };

			return false;
		} finally {
			working = false;
		}
	}

	async function move(stateId: string) {
		if (!issue || issue.state.id === stateId) return;

		if (!(await patch({ stateId })) && failure?.kind === "children_open") {
			pendingStateId = stateId;
		}
	}

	async function setPriority(priority: IssuePriority) {
		if (!issue || issue.priority === priority) return;

		await patch({ priority });
	}

	async function setAssignee(accountId: string) {
		if (!issue) return;

		await patch(accountId === "" ? { clear: ["assignee"] } : { assigneeId: accountId });
	}

	async function setCycle(cycleId: string) {
		if (!issue || (issue.cycleId ?? "") === cycleId) return;

		await patch(cycleId === "" ? { clear: ["cycle"] } : { cycleId });
	}

	async function setProject(projectId: string) {
		if (!issue || (issue.projectId ?? "") === projectId) return;

		await patch(projectId === "" ? { clear: ["project"] } : { projectId });
	}

	async function addRelation(
		kind: IssueRelationKind,
		counterpartId: string,
		closeDuplicate: boolean
	) {
		if (!issue) return;

		working = true;
		failure = null;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/issues/{issueId}/relations", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { kind, issueId: counterpartId, closeDuplicate },
			});

			if (error) failure = readIssueFailure(error);

			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function removeRelation(relationId: string) {
		if (!issue) return;

		working = true;
		failure = null;

		try {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/issues/{issueId}/relations/{relationId}",
				{
					params: {
						path: { workspaceId: data.workspace.id, issueId: issue.id, relationId },
					},
				}
			);

			if (error) failure = readIssueFailure(error);

			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function setParent(parentId: string | null) {
		if (!issue) return;

		working = true;
		failure = null;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/issues/{issueId}/parent", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { expectedVersion: issue.version, parentId },
			});

			if (error) failure = readIssueFailure(error);

			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function completeAnyway() {
		if (!issue || !pendingStateId) return;

		const stateId = pendingStateId;
		pendingStateId = "";

		await patch({ stateId, acknowledgeOpenChildren: true });
	}

	async function setStatus(status: "active" | "archived" | "pending_deletion") {
		if (!issue) return;

		working = true;
		failure = null;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/issues/{issueId}/status", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { expectedVersion: issue.version, status },
			});

			if (error) failure = readIssueFailure(error);

			await invalidateAll();
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function moveToTeam(teamId: string, acknowledgeLabelLoss: boolean) {
		if (!issue || issue.teamId === teamId) return;

		working = true;
		failure = null;

		try {
			const { data: moved, error } = await api.POST(
				"/workspaces/{workspaceId}/issues/{issueId}/team",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: { expectedVersion: issue.version, teamId, acknowledgeLabelLoss },
				}
			);

			if (error) {
				failure = readIssueFailure(error);
				pendingTeamId = failure.kind === "labels_out_of_scope" ? teamId : "";

				return;
			}

			pendingTeamId = "";

			if (moved) {
				await goto(at(`/issues/${moved.reference}`), { invalidateAll: true });
			}
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function comment(
		body: string,
		mentions: MentionTarget[],
		attachmentIds: string[],
		parentCommentId?: string
	): Promise<void> {
		if (!issue) return;

		await act(async () => {
			const { data: posted, error } = await api.POST(
				"/workspaces/{workspaceId}/issues/{issueId}/comments",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: { body, parentCommentId, mentions, attachmentIds },
				}
			);

			if (error || !posted) {
				commentFailure = readCommentFailure(error);

				return;
			}

			unreachable = posted.unreachable;
			loadedComments = null;
			commentUploads = [];
			await invalidateAll();
		});
	}

	async function editComment(commentId: string, body: string): Promise<void> {
		if (!issue) return;

		await act(async () => {
			const { error } = await api.PATCH(
				"/workspaces/{workspaceId}/issues/{issueId}/comments/{commentId}",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id, commentId } },
					body: { body },
				}
			);

			if (error) {
				commentFailure = readCommentFailure(error);

				return;
			}

			loadedComments = null;
			await invalidateAll();
		});
	}

	async function removeComment(commentId: string): Promise<void> {
		if (!issue) return;

		await act(async () => {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/issues/{issueId}/comments/{commentId}",
				{ params: { path: { workspaceId: data.workspace.id, issueId: issue.id, commentId } } }
			);

			if (error) {
				commentFailure = readCommentFailure(error);

				return;
			}

			loadedComments = null;
			await invalidateAll();
		});
	}

	async function react(commentId: string, reaction: CommentReaction, on: boolean): Promise<void> {
		if (!issue) return;

		const path = { workspaceId: data.workspace.id, issueId: issue.id, commentId, reaction };

		await act(async () => {
			const { error } = on
				? await api.PUT(
						"/workspaces/{workspaceId}/issues/{issueId}/comments/{commentId}/reactions/{reaction}",
						{ params: { path } }
					)
				: await api.DELETE(
						"/workspaces/{workspaceId}/issues/{issueId}/comments/{commentId}/reactions/{reaction}",
						{ params: { path } }
					);

			if (error) {
				commentFailure = readCommentFailure(error);

				return;
			}

			loadedComments = null;
			await invalidateAll();
		});
	}

	async function moreComments(): Promise<void> {
		const base = thread;

		if (!issue || base.kind !== "ready" || !base.nextCursor) return;

		await act(async () => {
			const { data: page, error } = await api.GET(
				"/workspaces/{workspaceId}/issues/{issueId}/comments",
				{
					params: {
						path: { workspaceId: data.workspace.id, issueId: issue.id },
						query: { cursor: base.nextCursor },
					},
				}
			);

			if (error || !page) {
				commentFailure = readCommentFailure(error);

				return;
			}

			loadedComments = {
				kind: "ready",
				comments: [...base.comments, ...page.comments],
				nextCursor: page.nextCursor,
			};
		});
	}

	async function moreActivity(): Promise<void> {
		const base = activity;

		if (!issue || base.kind !== "ready" || !base.nextCursor) return;

		await act(async () => {
			const { data: page, error } = await api.GET(
				"/workspaces/{workspaceId}/issues/{issueId}/activity",
				{
					params: {
						path: { workspaceId: data.workspace.id, issueId: issue.id },
						query: { cursor: base.nextCursor },
					},
				}
			);

			if (error || !page) {
				commentFailure = readCommentFailure(error);

				return;
			}

			loadedActivity = {
				kind: "ready",
				events: [...base.events, ...page.events],
				nextCursor: page.nextCursor,
			};
		});
	}

	async function act(run: () => Promise<void>): Promise<void> {
		working = true;
		commentFailure = null;

		try {
			await run();
		} catch {
			commentFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	function replaceTask(into: "comment" | "body", task: UploadTask) {
		const list = into === "comment" ? commentUploads : bodyUploads;
		const next = list.some((entry) => entry.id === task.id)
			? list.map((entry) => (entry.id === task.id ? task : entry))
			: [...list, task];

		if (into === "comment") {
			commentUploads = next;
		} else {
			bodyUploads = next;
		}
	}

	function begin(into: "comment" | "body", files: File[]) {
		if (!issue) return;

		attachmentFailure = null;

		for (const file of files) {
			const taskId = `upload-${(uploadSequence += 1)}`;
			const task = newTask(taskId, file);

			sources.set(taskId, file);
			replaceTask(into, task);

			void run(into, taskId, file, task);
		}
	}

	async function run(into: "comment" | "body", taskId: string, file: File, task: UploadTask) {
		if (!issue) return;

		await upload(
			{ workspaceId: data.workspace.id, issueId: issue.id },
			file,
			task,
			(next) => {
				replaceTask(into, next);

				if (next.state === "done" && next.attachment) {
					if (into === "body") {
						$formData.description = joined($formData.description, attachmentMarkdown(next.attachment));
					}

					void invalidateAll();
				}
			},
			(abort) => aborts.set(taskId, abort)
		);
	}

	function joined(body: string, markdown: string): string {
		if (body.trim() === "") return markdown;

		return body.endsWith("\n") ? body + markdown : `${body}\n${markdown}`;
	}

	function cancelUpload(taskId: string) {
		aborts.get(taskId)?.();
	}

	function retryUpload(into: "comment" | "body", taskId: string) {
		const file = sources.get(taskId);

		if (!file) return;

		const task = newTask(taskId, file);

		replaceTask(into, task);
		void run(into, taskId, file, task);
	}

	function dismissUpload(into: "comment" | "body", taskId: string) {
		const list = into === "comment" ? commentUploads : bodyUploads;
		const next = list.filter((entry) => entry.id !== taskId);

		if (into === "comment") {
			commentUploads = next;
		} else {
			bodyUploads = next;
		}
	}

	async function removeAttachment(attachmentId: string) {
		if (!issue) return;

		await act(async () => {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/issues/{issueId}/attachments/{attachmentId}",
				{
					params: {
						path: { workspaceId: data.workspace.id, issueId: issue.id, attachmentId },
					},
				}
			);

			if (error) {
				attachmentFailure = readAttachmentFailure(error);

				return;
			}

			await invalidateAll();
		});
	}

	function when(timestamp: string): string {
		return onDateAndTime(timestamp, data.workspace.timezone);
	}
</script>

<svelte:head>
	<title>{issue ? `${issue.reference} · ${issue.title}` : "Issue"} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<List class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<a
				href={at("/issues")}
				class="text-md font-medium tracking-snug whitespace-nowrap text-muted-foreground transition-colors duration-110 ease-out hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
			>
				Issues
			</a>
			{#if issue}
				<span class="text-md text-muted-foreground" aria-hidden="true">/</span>
				<span class="font-mono text-xs text-muted-foreground">{issue.reference}</span>
				<Button
					class="ml-auto shrink-0"
					variant={following ? "secondary" : "ghost"}
					size="sm"
					disabled={followWorking}
					aria-pressed={following}
					onclick={toggleFollow}
				>
					{#if following}
						<BellRing class="size-icon-row" aria-hidden="true" />
						Following
					{:else}
						<BellOff class="size-icon-row" aria-hidden="true" />
						Not following
					{/if}
				</Button>
			{/if}
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

			{#if detail.kind === "loading"}
				<div class="h-40 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if detail.kind === "not_found"}
				<div class="flex flex-col gap-2">
					<h1 class="text-md font-medium tracking-snug text-ink-900">No issue here</h1>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						There is no issue at this address, or it belongs to a team you are not on.
					</p>
					<div>
						<Button variant="secondary" size="sm" href={at("/issues")}>Back to issues</Button>
					</div>
				</div>
			{:else if detail.kind === "unavailable" || !issue || !ready}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>We could not load this issue</Alert.Title>
					<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
				</Alert.Root>
			{:else}
				{#if labelFailure}
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>That did not work</Alert.Title>
						<Alert.Description>{labelFailureMessage(labelFailure)}</Alert.Description>
					</Alert.Root>
				{/if}

				{#if failure}
					<Alert.Root variant={failure.kind === "labels_out_of_scope" ? "default" : "destructive"}>
						<CircleX aria-hidden="true" />
						<Alert.Title>
							{failure.kind === "stale"
								? "Someone got there first"
								: failure.kind === "labels_out_of_scope"
									? "This move drops labels"
									: failure.kind === "children_open"
										? "Work beneath this is unfinished"
										: "That did not work"}
						</Alert.Title>
						<Alert.Description>
							{issueFailureMessage(failure)}
							{#if failure.kind === "labels_out_of_scope" && pendingTeamId}
								<span class="mt-2 block">
									<Button
										variant="secondary"
										size="sm"
										disabled={working}
										onclick={() => moveToTeam(pendingTeamId, true)}
									>
										Move anyway
									</Button>
								</span>
							{/if}
							{#if failure.kind === "children_open" && pendingStateId}
								<span class="mt-2 block">
									<Button
										variant="secondary"
										size="sm"
										disabled={working}
										onclick={completeAnyway}
									>
										Finish it anyway
									</Button>
								</span>
							{/if}
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if issue.blocked}
					<Alert.Root>
						<Link2 aria-hidden="true" />
						<Alert.Title>Blocked</Alert.Title>
						<Alert.Description>
							Something this issue is blocked by is still open. It cannot be started until that
							clears.
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if issue.status !== "active"}
					<Alert.Root>
						<Archive aria-hidden="true" />
						<Alert.Title>
							{issue.status === "archived" ? "Archived" : "Deleted"}
						</Alert.Title>
						<Alert.Description>
							{issue.status === "archived"
								? `Archived ${issue.archivedAt ? when(issue.archivedAt) : ""}. It keeps its reference and stays readable.`
								: "This issue is scheduled to be removed for good. Restore it to keep it."}
						</Alert.Description>
					</Alert.Root>
				{/if}

				{#if editing}
					<form method="POST" use:enhance class="flex flex-col gap-4">
						<Form.Field {form} name="title">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Title</Form.Label>
									<Input {...props} bind:value={$formData.title} disabled={$submitting} />
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>

						<Form.Field {form} name="description">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Description</Form.Label>
									<Textarea
										{...props}
										bind:value={$formData.description}
										rows={10}
										disabled={$submitting}
										class="font-mono text-sm"
									/>
								{/snippet}
							</Form.Control>
							<Form.Description>Markdown. Stored exactly as you type it.</Form.Description>
							<Form.FieldErrors />
						</Form.Field>

						<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
							<Form.Field {form} name="estimate">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Estimate</Form.Label>
										<Input
											{...props}
											bind:value={$formData.estimate}
											inputmode="numeric"
											placeholder="Points"
											disabled={$submitting}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>

							<Form.Field {form} name="dueOn">
								<Form.Control>
									{#snippet children({ props })}
										<Form.Label>Due date</Form.Label>
										<Input
											{...props}
											type="date"
											bind:value={$formData.dueOn}
											disabled={$submitting}
										/>
									{/snippet}
								</Form.Control>
								<Form.FieldErrors />
							</Form.Field>
						</div>

						<div class="flex items-center gap-2">
							<Form.Button disabled={$submitting || working}>
								{$submitting ? "Saving" : "Save"}
							</Form.Button>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								disabled={$submitting}
								onclick={() => (editing = false)}
							>
								Cancel
							</Button>
						</div>
					</form>
				{:else}
					<div class="flex flex-col gap-2">
						<div class="flex items-start justify-between gap-3">
							<span class="font-mono text-xs text-muted-foreground">{issue.reference}</span>
							<Button
								variant="ghost"
								size="sm"
								disabled={working}
								onclick={() => (editing = true)}
							>
								<Pencil aria-hidden="true" />
								Edit
							</Button>
						</div>
						<h1 class="text-lg font-medium tracking-snug text-ink-900 text-pretty">
							{issue.title}
						</h1>
					</div>

					<section class="flex flex-col gap-2">
						<h2 class="text-sm font-medium text-ink-900">Description</h2>
						{#if issue.description.trim()}
							<Markdown source={issue.description} />
						{:else}
							<p class="text-sm text-muted-foreground">No description yet.</p>
						{/if}
					</section>
				{/if}

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">State</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<StatusIcon category={issue.state.category} decorative />
										{issue.state.name}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start">
								<DropdownMenu.RadioGroup
									value={issue.state.id}
									onValueChange={(value) => move(value)}
								>
									<DropdownMenu.GroupHeading>Move to</DropdownMenu.GroupHeading>
									{#each ready.states as state (state.id)}
										<DropdownMenu.RadioItem value={state.id}>
											{state.name}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Priority</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<PriorityIcon priority={issue.priority} />
										{priorityLabel(issue.priority)}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start">
								<DropdownMenu.RadioGroup
									value={issue.priority}
									onValueChange={(value) => setPriority(value as IssuePriority)}
								>
									{#each priorities as choice (choice.value)}
										<DropdownMenu.RadioItem value={choice.value}>
											{choice.label}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Assignee</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<UserRound aria-hidden="true" />
										{assigneeName || "Unassigned"}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
								<DropdownMenu.RadioGroup
									value={issue.assigneeAccountId ?? ""}
									onValueChange={(value) => setAssignee(value)}
								>
									<DropdownMenu.RadioItem value="">Unassigned</DropdownMenu.RadioItem>
									<DropdownMenu.Separator />
									{#each ready.members as member (member.accountId)}
										<DropdownMenu.RadioItem value={member.accountId}>
											{member.displayName || member.email}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Cycle</h2>
					<div>
						{#if ready.cycles.length === 0}
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								{issue.teamKey} does not use cycles.
							</p>
						{:else}
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button {...props} variant="outline" size="sm" disabled={working}>
											<Layers aria-hidden="true" />
											{issue.cycleNumber ? `Cycle ${issue.cycleNumber}` : "No cycle"}
											<ChevronDown aria-hidden="true" />
										</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
									<DropdownMenu.RadioGroup
										value={issue.cycleId ?? ""}
										onValueChange={(value) => setCycle(value)}
									>
										<DropdownMenu.RadioItem value="">No cycle</DropdownMenu.RadioItem>
										<DropdownMenu.Separator />
										{#each ready.cycles as cycle (cycle.id)}
											<DropdownMenu.RadioItem value={cycle.id}>
												{cycle.name} · {cycleWindow(cycle.startsOn, cycle.endsOn)}
											</DropdownMenu.RadioItem>
										{/each}
									</DropdownMenu.RadioGroup>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						{/if}
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Project</h2>
					<div>
						{#if ready.projects.length === 0}
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								{data.workspace.name} has no projects yet.
							</p>
						{:else}
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button {...props} variant="outline" size="sm" disabled={working}>
											<Target aria-hidden="true" />
											{issue.projectName || "No project"}
											<ChevronDown aria-hidden="true" />
										</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
									<DropdownMenu.RadioGroup
										value={issue.projectId ?? ""}
										onValueChange={(value) => setProject(value)}
									>
										<DropdownMenu.RadioItem value="">No project</DropdownMenu.RadioItem>
										<DropdownMenu.Separator />
										{#each ready.projects as project (project.id)}
											<DropdownMenu.RadioItem value={project.id}>
												{project.name}
											</DropdownMenu.RadioItem>
										{/each}
									</DropdownMenu.RadioGroup>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						{/if}
					</div>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Estimate and due date</h2>
					<dl class="flex flex-wrap gap-x-8 gap-y-2 text-sm">
						<div class="flex items-baseline gap-2">
							<dt class="text-muted-foreground">Estimate</dt>
							<dd class="font-mono text-ink-900">
								{issue.estimate ? `${issue.estimate} ${issue.estimate === 1 ? "point" : "points"}` : "None"}
							</dd>
						</div>
						<div class="flex items-baseline gap-2">
							<dt class="text-muted-foreground">Due</dt>
							<dd
								class="font-mono {issue.dueOn && overdue(issue.dueOn, data.now)
									? 'text-priority-urgent'
									: 'text-ink-900'}"
							>
								{issue.dueOn ? dueLabel(issue.dueOn, data.now, data.workspace.timezone) : "No date"}
							</dd>
						</div>
					</dl>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Team</h2>
					<div>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<Users aria-hidden="true" />
										{issue.teamKey}
										<ChevronDown aria-hidden="true" />
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start">
								<DropdownMenu.RadioGroup
									value={issue.teamId}
									onValueChange={(value) => moveToTeam(value, false)}
								>
									<DropdownMenu.GroupHeading>Move to</DropdownMenu.GroupHeading>
									{#each teams as team (team.id)}
										<DropdownMenu.RadioItem value={team.id}>
											{team.key} · {team.name}
										</DropdownMenu.RadioItem>
									{/each}
								</DropdownMenu.RadioGroup>
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						{issue.reference} keeps its reference wherever it goes. Its state becomes the matching one
						on the new team.
					</p>
				</section>

				<IssueParent
					{issue}
					candidates={ready.candidates}
					{at}
					{working}
					onchoose={setParent}
				/>

				<IssueChildren children={ready.children} progress={ready.childProgress} {at} />

				<IssueRelations
					{issue}
					groups={ready.relations}
					candidates={ready.candidates}
					{at}
					{working}
					onadd={addRelation}
					onremove={removeRelation}
				/>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">Labels</h2>

					<div class="flex flex-wrap items-center gap-2">
						{#each labels as label (label.id)}
							<Tag name={label.name} color={label.color} />
						{/each}

						{#if labels.length === 0}
							<span class="text-sm text-muted-foreground">None yet.</span>
						{/if}

						<DropdownMenu.Root>
							<DropdownMenu.Trigger>
								{#snippet child({ props })}
									<Button {...props} variant="outline" size="sm" disabled={working}>
										<Tags aria-hidden="true" />
										{working ? "Saving" : "Labels"}
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content align="start" class="max-h-80 overflow-auto">
								{#if available.length === 0}
									<DropdownMenu.Label>No labels apply to this team yet</DropdownMenu.Label>
								{:else}
									{#each sections as section (section.group?.id ?? "ungrouped")}
										{#if section.group}
											<DropdownMenu.RadioGroup
												value={labels.find((label) => label.groupId === section.group!.id)?.id ??
													""}
												onValueChange={(value) => {
													const chosen = section.labels.find((label) => label.id === value);
													if (chosen) submit(toggled(labels, chosen));
												}}
											>
												<DropdownMenu.GroupHeading>
													{section.group.name} · one only
												</DropdownMenu.GroupHeading>
												{#each section.labels as label (label.id)}
													<DropdownMenu.RadioItem value={label.id}>
														{label.name}
													</DropdownMenu.RadioItem>
												{/each}
											</DropdownMenu.RadioGroup>
											<DropdownMenu.Separator />
										{:else if section.labels.length > 0}
											<DropdownMenu.Group>
												<DropdownMenu.GroupHeading>Any number</DropdownMenu.GroupHeading>
												{#each section.labels as label (label.id)}
													<DropdownMenu.CheckboxItem
														checked={labels.some((chosen) => chosen.id === label.id)}
														onCheckedChange={() => submit(toggled(labels, label))}
													>
														{label.name}
													</DropdownMenu.CheckboxItem>
												{/each}
											</DropdownMenu.Group>
										{/if}
									{/each}
								{/if}
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>

					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Labels are defined in
						<a
							href={at("/settings/labels")}
							class="text-link underline-offset-2 hover:text-link-hover hover:underline"
						>
							workspace settings
						</a>. A label narrowed to another team does not appear here.
					</p>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">History</h2>
					<dl class="flex flex-col gap-1 text-sm">
						<div class="flex flex-wrap items-baseline gap-x-2">
							<dt class="text-muted-foreground">Raised</dt>
							<dd class="font-mono text-xs text-ink-900">
								<time datetime={issue.createdAt}>{when(issue.createdAt)}</time>
							</dd>
						</div>
						<div class="flex flex-wrap items-baseline gap-x-2">
							<dt class="text-muted-foreground">In {issue.state.name} since</dt>
							<dd class="font-mono text-xs text-ink-900">
								<time datetime={issue.stateEnteredAt}>{when(issue.stateEnteredAt)}</time>
							</dd>
						</div>
						{#if issue.completedAt}
							<div class="flex flex-wrap items-baseline gap-x-2">
								<dt class="text-muted-foreground">Finished</dt>
								<dd class="font-mono text-xs text-ink-900">
									<time datetime={issue.completedAt}>{when(issue.completedAt)}</time>
								</dd>
							</div>
						{/if}
					</dl>
				</section>

				<section class="flex flex-col gap-2">
					<h2 class="text-sm font-medium text-ink-900">This issue</h2>
					<div class="flex flex-wrap items-center gap-2">
						{#if issue.status === "active"}
							<Button
								variant="outline"
								size="sm"
								disabled={working}
								onclick={() => setStatus("archived")}
							>
								<Archive aria-hidden="true" />
								Archive
							</Button>
						{:else}
							<Button
								variant="outline"
								size="sm"
								disabled={working}
								onclick={() => setStatus("active")}
							>
								<ArchiveRestore aria-hidden="true" />
								{issue.status === "archived" ? "Take out of the archive" : "Restore"}
							</Button>
						{/if}

						{#if issue.status !== "pending_deletion"}
							<Button
								variant="ghost"
								size="sm"
								disabled={working}
								onclick={() => setStatus("pending_deletion")}
							>
								<Trash2 aria-hidden="true" />
								Delete
							</Button>
						{/if}
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						An archived issue is kept for good and stays readable. A deleted one is removed for
						good after 30 days, and can be restored until then. Neither ever frees its reference.
					</p>
				</section>

				<section class="flex flex-col gap-3">
					<h2 class="text-sm font-medium text-ink-900">Files</h2>

					{#if attachmentFailure}
						<Alert.Root variant="destructive">
							<CircleX aria-hidden="true" />
							<Alert.Title>That did not stick</Alert.Title>
							<Alert.Description>{attachmentFailureMessage(attachmentFailure)}</Alert.Description>
						</Alert.Root>
					{/if}

					<AttachmentList panel={attachments} {working} onremove={removeAttachment} />

					<UploadList
						uploads={attachmentPreview?.bodyUploads ?? bodyUploads}
						oncancel={cancelUpload}
						onretry={(taskId) => retryUpload("body", taskId)}
						ondismiss={(taskId) => dismissUpload("body", taskId)}
					/>

					<div>
						<AttachmentPicker
							disabled={working}
							label="Attach a file"
							onfiles={(files) => begin("body", files)}
						/>
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Anyone who can open this issue can open its files, and nobody else. An image is shown
						where it is written into the description or a comment.
					</p>
				</section>

				<CommentThreadView
					{thread}
					uploads={shownCommentUploads}
					onfiles={(files) => begin("comment", files)}
					oncancelupload={cancelUpload}
					onretryupload={(taskId) => retryUpload("comment", taskId)}
					ondismissupload={(taskId) => dismissUpload("comment", taskId)}
					members={ready.members}
					teams={data.teams ?? []}
					accountId={data.member.id}
					{when}
					working={working || Boolean(commentPreview?.working)}
					failure={commentFailure ?? commentPreview?.failure ?? null}
					unreachable={commentPreview?.unreachable ?? unreachable}
					onpost={comment}
					onedit={editComment}
					onremove={removeComment}
					onreact={react}
					onmore={moreComments}
				/>

				<section class="flex flex-col gap-3">
					<h2 class="text-sm font-medium text-ink-900">Activity</h2>
					<ActivityFeedView
						feed={activity}
						{when}
						{working}
						hideComments
						onmore={moreActivity}
					/>
				</section>
			{/if}
		</div>
	</div>
</div>
