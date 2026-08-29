<script lang="ts">
	import type { Snippet } from "svelte";
	import { goto, invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import Archive from "@lucide/svelte/icons/archive";
	import BellOff from "@lucide/svelte/icons/bell-off";
	import BellRing from "@lucide/svelte/icons/bell-ring";
	import CalendarDays from "@lucide/svelte/icons/calendar-days";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronLeft from "@lucide/svelte/icons/chevron-left";
	import ChevronUp from "@lucide/svelte/icons/chevron-up";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleDot from "@lucide/svelte/icons/circle-dot";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Copy from "@lucide/svelte/icons/copy";
	import CornerDownLeft from "@lucide/svelte/icons/corner-down-left";
	import Ellipsis from "@lucide/svelte/icons/ellipsis";
	import Folder from "@lucide/svelte/icons/folder";
	import Info from "@lucide/svelte/icons/info";
	import Link2 from "@lucide/svelte/icons/link-2";
	import Pencil from "@lucide/svelte/icons/pencil";
	import Plus from "@lucide/svelte/icons/plus";
	import Tags from "@lucide/svelte/icons/tags";
	import Trash2 from "@lucide/svelte/icons/trash-2";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import UserRound from "@lucide/svelte/icons/user-round";
	import Users from "@lucide/svelte/icons/users";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as Popover from "$lib/components/ui/popover/index.js";
	import { Calendar } from "$lib/components/ui/calendar/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import Kbd from "$lib/components/norn/kbd.svelte";
	import ProgressBar from "$lib/components/norn/progress-bar.svelte";
	import StatusIcon from "$lib/components/norn/status-icon.svelte";
	import Tag from "$lib/components/norn/tag.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import IssueField from "$lib/issues/issue-field.svelte";
	import DelegateDialog from "$lib/agents/delegate-dialog.svelte";
	import DelegationField from "$lib/agents/delegation-field.svelte";
	import {
		agentMembers,
		delegationFailureMessage,
		readDelegationFailure,
		type DelegationFailure,
		type DelegationPanel,
	} from "$lib/agents/delegation";
	import { totalIssues } from "$lib/issues/board";
	import { assignees } from "$lib/workspace/members";
	import { initialsOf } from "$lib/team/members";
	import { Button } from "$lib/components/ui/button/index.js";
	import { api } from "$lib/api";
	import { useShortcuts } from "$lib/shortcuts/registry.svelte";
	import { showToast } from "$lib/toast/toasts";
	import { keys } from "$lib/api/keys";
	import { useRealtime } from "$lib/realtime/connection.svelte";
	import { calendarDate, cycleWindow, dueLabel, onDate, onDateAndTime, overdue } from "$lib/time";
	import Markdown from "$lib/issues/markdown.svelte";
	import {
		issueFailureMessage,
		priorities,
		priorityLabel,
		readIssueFailure,
		type IssueFailure,
		type IssuePriority,
		type IssueRelationKind,
	} from "$lib/issues/issues";
	import { issueEditSchema } from "$lib/issues/issue-schema";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import Layers from "@lucide/svelte/icons/layers";
	import Target from "@lucide/svelte/icons/target";
	import PriorityIcon from "$lib/components/norn/priority-icon.svelte";
	import IssueChildren from "$lib/issues/issue-children.svelte";
	import NewIssueDialog from "$lib/issues/new-issue-dialog.svelte";
	import type { CreationOutcome } from "$lib/issues/creating";
	import type { NewIssueInput } from "$lib/issues/new-issue-schema";
	import IssueRelations from "$lib/issues/issue-relations.svelte";
	import CommentThreadView from "$lib/comments/comment-thread.svelte";
	import {
		actorLabel,
		changeLine,
		readable,
		type ActivityFeed,
	} from "$lib/activity/activity";
	import AttachmentList from "$lib/attachments/attachment-list.svelte";
	import QuestionList from "$lib/questions/question-list.svelte";
	import RunRow from "$lib/executions/run-row.svelte";
	import RepoChange from "$lib/executions/repo-change.svelte";
	import {
		changeStatLine,
		changeTotals,
		type IssueRepositoryChange,
	} from "$lib/executions/executions";
	import type { Execution } from "$lib/executions/executions";
	import type { IssueQuestion } from "$lib/questions/questions";
	import CodeLinkPanel from "$lib/source-control/code-link-panel.svelte";

	import {
		conflictSummary,
		deploymentLabel,
		releaseLabel,
		sourceControlFailure,
		failureMessage as codeFailureMessage,
		type CodeLink,
		type SourceControlFailure,
	} from "$lib/source-control/source-control";
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
	import { parseDate } from "@internationalized/date";
	import { workspacePath } from "$lib/workspace/navigation";
	import {
		activityPreviewStates,
		attachmentPreviewStates,
		commentPreviewStates,
		delegationPreviewStates,
		issueDetailPreviewStates,
	} from "./preview";
	import type { IssueDetail } from "./+page.server";
	import type { Issue, IssueComment } from "$lib/realtime/connection.svelte";
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
	const delegationPreview = $derived(
		import.meta.env.DEV
			? delegationPreviewStates[page.url.searchParams.get("delegation") ?? ""]
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
	let pendingTeamId = $state("");
	let commentFailure = $state<CommentFailure | null>(null);
	let unreachable = $state.raw<CommentMention[]>([]);
	let loadedComments = $state.raw<CommentThread | null>(null);
	let loadedActivity = $state.raw<ActivityFeed | null>(null);
	let commentUploads = $state.raw<UploadTask[]>([]);
	let bodyUploads = $state.raw<UploadTask[]>([]);
	let attachmentFailure = $state<AttachmentFailure | null>(null);
	let codeLinkFailure = $state<SourceControlFailure | null>(null);
	let removedCodeLinks = $state.raw<string[]>([]);
	let unlinking = $state(false);
	let branchName = $state("");
	let branchCopied = $state(false);
	let copyingBranch = $state(false);
	let togglingAutomation = $state(false);
	let automationOverride = $state.raw<boolean | null>(null);

	let uploadSequence = 0;

	const aborts = new Map<string, () => void>();
	const sources = new Map<string, File>();
	let pendingStateId = $state("");
	let questionFailed = $state(false);
	let followWorking = $state(false);
	let pushed = $state.raw<{ source: unknown; issue: Issue } | null>(null);
	let pushedComments = $state.raw<{ source: unknown; comments: IssueComment[] }>({
		source: null,
		comments: [],
	});

	const realtime = useRealtime();

	$effect(() => {
		if (!realtime || !issue) return;

		const openIssue = issue.id;

		return realtime.on((event) => {
			if (event.issueId !== openIssue) return;

			if (event.kind === "issue.updated") {
				pushed = { source: ready, issue: event.payload as Issue };
			}

			if (event.kind === "comment.posted") {
				const held = pushedComments.source === ready ? pushedComments.comments : [];

				pushedComments = { source: ready, comments: [...held, event.payload as IssueComment] };
			}

			if (event.kind === "comment.edited" || event.kind === "comment.deleted") {
				realtime.refetch(keys.issue(event.issueId));
			}
		});
	});


	async function toggleFollow() {
		if (!issue) return;

		followWorking = true;

		try {
			const { error } = await api.PUT("/workspaces/{workspaceId}/issues/{issueId}/follow", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { state: following ? "muted" : "following" },
			});

			if (error) return;

			announce(
				following
					? `You will not be notified about ${issue.reference} unless someone names you.`
					: `You are following ${issue.reference}.`
			);

			await invalidate(keys.page(page.route.id));
		} finally {
			followWorking = false;
		}
	}

	let delegating = $state(false);
	let delegated = $state<DelegationPanel | null>(null);
	let delegationFailed = $state<DelegationFailure | null>(null);

	const detail = $derived<IssueDetail>(preview?.detail ?? data.detail);
	const ready = $derived(detail.kind === "ready" ? detail : null);
	const delegation = $derived<DelegationPanel>(
		delegationPreview?.panel ?? delegated ?? (ready ? ready.delegation : { kind: "loading" })
	);
	const agents = $derived(agentMembers(ready?.members ?? []));
	const people = $derived(assignees(ready?.members ?? []));
	const delegationFailure = $derived(delegationFailed ?? delegationPreview?.failure ?? null);
	const issue = $derived((pushed?.source === ready ? pushed.issue : null) ?? ready?.issue ?? null);
	const assigned = $derived(delegationPreview?.assigned ?? Boolean(issue?.assigneeAccountId));
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
	const codeLinks = $derived<CodeLink[]>(
		(ready?.codeLinks ?? []).filter((link) => !removedCodeLinks.includes(link.id))
	);
	const questions = $derived<IssueQuestion[]>(ready ? ready.questions : []);
	const runs = $derived<Execution[]>(ready ? ready.runs : []);
	const changed = $derived<IssueRepositoryChange[]>(ready ? (ready.changeset?.repositories ?? []) : []);
	const automationSuppressed = $derived(automationOverride ?? false);
	const mirrorConflicts = $derived(ready?.mirrorConflicts ?? []);
	const shipping = $derived(ready?.shipping ?? { releases: [], deployments: [] });
	const activity = $derived<ActivityFeed>(
		activityPreview?.feed ?? loadedActivity ?? ready?.activity ?? { kind: "loading" }
	);
	const shownCommentUploads = $derived(attachmentPreview?.uploads ?? commentUploads);

	// The branch name is asked for rather than built here: the template belongs to the team
	// and only the server knows it.
	async function copyBranchName() {
		if (!issue) return;

		copyingBranch = true;
		branchCopied = false;
		codeLinkFailure = null;

		const { data: found, error } = await api.GET(
			"/workspaces/{workspaceId}/issues/{issueId}/branch-name",
			{ params: { path: { workspaceId: data.workspace.id, issueId: issue.id } } },
		);

		copyingBranch = false;

		if (error || !found) {
			codeLinkFailure = sourceControlFailure(error);

			return;
		}

		branchName = found.branch;

		try {
			await navigator.clipboard.writeText(found.branch);

			branchCopied = true;
		} catch {
			// Writing to the clipboard is refused without a user gesture or over plain http.
			// The name is shown below the button either way, so there is still something to copy.
			branchCopied = false;
		}
	}

	async function setAutomation(suppressed: boolean) {
		if (!issue) return;

		togglingAutomation = true;
		codeLinkFailure = null;

		const { error } = await api.PUT(
			"/workspaces/{workspaceId}/issues/{issueId}/code-automation",
			{
				params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
				body: { suppressed },
			},
		);

		togglingAutomation = false;

		if (error) {
			codeLinkFailure = sourceControlFailure(error);

			return;
		}

		automationOverride = suppressed;
	}

	async function unlinkCode(link: CodeLink) {
		if (!issue) return;

		unlinking = true;
		codeLinkFailure = null;

		const { error } = await api.DELETE(
			"/workspaces/{workspaceId}/issues/{issueId}/code-links/{linkId}",
			{
				params: {
					path: { workspaceId: data.workspace.id, issueId: issue.id, linkId: link.id },
				},
			}
		);

		unlinking = false;

		if (error) {
			codeLinkFailure = sourceControlFailure(error);

			return;
		}

		removedCodeLinks = [...removedCodeLinks, link.id];
	}

	const thread = $derived<CommentThread>(
		withPushed(commentPreview?.thread ?? loadedComments ?? ready?.comments ?? { kind: "loading" })
	);

	function withPushed(base: CommentThread): CommentThread {
		if (pushedComments.source !== ready || pushedComments.comments.length === 0) return base;

		const existing = base.kind === "ready" ? base.comments : [];
		const known = new Set(existing.map((comment) => comment.id));
		const arrived = pushedComments.comments.filter((comment) => !known.has(comment.id));

		if (arrived.length === 0) return base;

		return { kind: "ready", comments: [...existing, ...arrived] };
	}

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
				editingField = null;

				return;
			}

			if (await patch(body)) {
				editingField = null;
				announce(`Saved ${issue.reference}.`);

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
		if (!issue || editingField) return;

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
	async function recall() {
		if (working) return;

		working = true;
		delegationFailed = null;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/issues/{issueId}/delegation", {
				params: { path: { workspaceId: data.workspace.id, issueId: issue?.id ?? "" } },
			});

			if (error) {
				delegationFailed = readDelegationFailure(error);

				return;
			}

			delegated = { kind: "none" };

			await invalidate(keys.page(page.route.id));
		} catch {
			delegationFailed = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

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
				await invalidate(keys.page(page.route.id));

				return;
			}

			applied = next ?? [];
			announce(
				`This issue now carries ${applied.length} ${applied.length === 1 ? "label" : "labels"}.`
			);
			await invalidate(keys.page(page.route.id));
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
				await invalidate(keys.page(page.route.id));

				return false;
			}

			await invalidate(keys.page(page.route.id));

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

		const target = (ready?.states ?? []).find((state) => state.id === stateId);

		if (await patch({ stateId })) {
			announce(`Moved ${issue.reference} to ${target?.name ?? "another status"}`);

			return;
		}

		if (failure?.kind === "children_open") pendingStateId = stateId;
	}

	async function answerQuestion(question: IssueQuestion, answer: string): Promise<void> {
		if (!issue) return;

		working = true;
		questionFailed = false;

		try {
			const { error } = await api.POST(
				"/workspaces/{workspaceId}/issues/{issueId}/questions/{questionId}/answer",
				{
					params: {
						path: {
							workspaceId: data.workspace.id,
							issueId: issue.id,
							questionId: question.id,
						},
					},
					body: { answer },
				}
			);

			questionFailed = Boolean(error);

			await invalidate(keys.page(page.route.id));
		} catch {
			questionFailed = true;
		} finally {
			working = false;
		}
	}

	async function dismissQuestion(question: IssueQuestion): Promise<void> {
		if (!issue) return;

		working = true;
		questionFailed = false;

		try {
			const { error } = await api.POST(
				"/workspaces/{workspaceId}/issues/{issueId}/questions/{questionId}/dismiss",
				{
					params: {
						path: {
							workspaceId: data.workspace.id,
							issueId: issue.id,
							questionId: question.id,
						},
					},
				}
			);

			questionFailed = Boolean(error);

			await invalidate(keys.page(page.route.id));
		} catch {
			questionFailed = true;
		} finally {
			working = false;
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

			await invalidate(keys.page(page.route.id));
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

			await invalidate(keys.page(page.route.id));
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

			await invalidate(keys.page(page.route.id));
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

			await invalidate(keys.page(page.route.id));
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

			if (!("unreachable" in posted)) {
				commentFailure = readCommentFailure(undefined);

				return;
			}

			unreachable = posted.unreachable;
			loadedComments = null;
			commentUploads = [];
			await invalidate(keys.page(page.route.id));
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
			await invalidate(keys.page(page.route.id));
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
			await invalidate(keys.page(page.route.id));
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
			await invalidate(keys.page(page.route.id));
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

					void invalidate(keys.page(page.route.id));
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

			await invalidate(keys.page(page.route.id));
		});
	}

	function when(timestamp: string): string {
		return onDateAndTime(timestamp, data.workspace.timezone);
	}

	let editingField = $state<"title" | "description" | null>(null);
	let titleField = $state<HTMLInputElement | null>(null);
	let descriptionField = $state<HTMLTextAreaElement | null>(null);
	let parentPicking = $state(false);
	let pickingDue = $state(false);
	let addingChild = $state(false);
	let childPrefill = $state<Partial<NewIssueInput> | undefined>(undefined);
	const due = $derived(issue?.dueOn ? parseDate(issue.dueOn) : undefined);
	let shown = $state<"all" | "comments">("all");

	const role = $derived(
		ready?.members.find((member) => member.accountId === data.member.id)?.role ?? "member"
	);
	const canEdit = $derived(Boolean(issue) && role !== "viewer" && issue?.status === "active");
	const closedIssue = $derived(
		issue?.state.category === "complete" || issue?.state.category === "abandoned"
	);

	function initials(name: string): string {
		return initialsOf(name);
	}

	function nameOf(accountId: string | undefined): string {
		if (!accountId) return "";

		const member = ready?.members.find((candidate) => candidate.accountId === accountId);

		return member?.displayName || member?.email || "";
	}

	async function settle(outcome: CreationOutcome) {
		if (outcome.kind === "refused") {
			announce(outcome.failure);

			if (outcome.input) {
				childPrefill = outcome.input;
				addingChild = true;
			}

			return;
		}

		await fileUnderThis(outcome.issue);
	}

	async function fileUnderThis(created: { id: string; reference: string; version: number }) {
		if (!issue) return;

		working = true;

		try {
			const { error } = await api.POST("/workspaces/{workspaceId}/issues/{issueId}/parent", {
				params: { path: { workspaceId: data.workspace.id, issueId: created.id } },
				body: { expectedVersion: created.version, parentId: issue.id },
			});

			if (error) failure = readIssueFailure(error);

			announce(`${created.reference} is filed under ${issue.reference}`);
			await invalidate(keys.page(page.route.id));
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function moveChild(child: Issue, stateId: string) {
		if (child.state.id === stateId) return;

		working = true;

		try {
			const { error } = await api.PATCH("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId: data.workspace.id, issueId: child.id } },
				body: { expectedVersion: child.version, stateId },
			});

			if (error) failure = readIssueFailure(error);

			await invalidate(keys.page(page.route.id));
		} catch {
			failure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function setEstimate(points: string) {
		if (!issue) return;

		await patch(points === "" ? { clear: ["estimate"] } : { estimate: Number(points) });
	}

	async function setDue(date: string) {
		if (!issue) return;

		await patch(date === "" ? { clear: ["dueOn"] } : { dueOn: date });
	}
	const dirty = $derived(
		Boolean(issue) &&
			(($formData.title !== issue?.title && editingField === "title") ||
				($formData.description !== issue?.description && editingField === "description"))
	);

	const watchers = $derived(
		(ready?.watchers ?? [])
			.map((accountId) => ({ accountId, name: nameOf(accountId) }))
			.filter((watcher) => watcher.name)
	);

	const reporter = $derived(
		ready?.members.find((member) => member.accountId === issue?.createdByAccountId)?.displayName ??
			"Someone"
	);
	const commentTotal = $derived(thread.kind === "ready" ? thread.comments.length : 0);
	const lastActivity = $derived(
		activity.kind === "ready" ? (activity.events.at(-1)?.createdAt ?? null) : null
	);
	const issueLink = $derived(
		issue ? `${page.url.host}${at(`/issues/${issue.reference}`)}` : ""
	);

	const order = $derived(ready?.candidates ?? []);
	const position = $derived(order.findIndex((candidate) => candidate.id === issue?.id));
	const previous = $derived(position > 0 ? order[position - 1] : null);
	const next = $derived(
		position >= 0 && position < order.length - 1 ? order[position + 1] : null
	);

	const reopenState = $derived(
		(ready?.states ?? []).find(
			(state) => state.category !== "complete" && state.category !== "abandoned"
		)
	);

	const events = $derived(
		activity.kind === "ready"
			? activity.events.filter((event) => readable(event)).map((event) => ({
					id: event.id,
					at: event.createdAt,
					line: `${actorLabel(event)} · ${event.changes.map(changeLine).join(", ")}`,
				}))
			: []
	);

	async function loadEarlier() {
		await moreComments();

		if (activity.kind === "ready" && activity.nextCursor) await moreActivity();
	}

	function announce(message: string) {
		showToast(message);
	}

	async function copy(text: string, said: string) {
		try {
			await navigator.clipboard.writeText(text);
			announce(said);
		} catch {
			announce("Your browser would not let us copy that");
		}
	}

	function startEditing(field: "title" | "description") {
		if (!canEdit || !issue) return;

		formData.update(
			(current) => ({ ...current, title: issue.title, description: issue.description }),
			{ taint: false }
		);
		editingField = field;
	}

	const editedField = $derived(
		editingField === "title" ? titleField : editingField === "description" ? descriptionField : null
	);

	$effect(() => {
		if (!editedField) return;

		editedField.focus();
		editedField.setSelectionRange(editedField.value.length, editedField.value.length);
	});

	function discard() {
		if (!issue) return;

		formData.update(
			(current) => ({ ...current, title: issue.title, description: issue.description }),
			{ taint: false }
		);
		editingField = null;
		failure = null;
	}

	async function reopen() {
		if (!reopenState) return;

		await move(reopenState.id);
	}

	async function duplicate() {
		if (!issue) return;

		working = true;

		try {
			const { data: copied, error } = await api.POST("/workspaces/{workspaceId}/issues", {
				params: { path: { workspaceId: data.workspace.id } },
				body: {
					teamId: issue.teamId,
					title: `${issue.title} (copy)`,
					description: issue.description,
					priority: issue.priority,
					labelIds: labels.map((label) => label.id),
					...(issue.projectId ? { projectId: issue.projectId } : {}),
				},
			});

			if (error || !copied) {
				failure = readIssueFailure(error);

				return;
			}

			await goto(at(`/issues/${copied.reference}`));
		} finally {
			working = false;
		}
	}

	function onPointerDown(event: PointerEvent) {
		if (editingField !== "description") return;

		const target = event.target as HTMLElement | null;

		if (target?.closest("[data-editing-region]")) return;

		if (dirty) {
			(document.getElementById("issue-edit-form") as HTMLFormElement | null)?.requestSubmit();

			return;
		}

		editingField = null;
	}

	const shortcuts = useShortcuts();

	$effect(() => {
		if (!canEdit || editingField) return;

		return shortcuts.register("issue-edit", () => startEditing("description"));
	});

	function onKey(event: KeyboardEvent) {
		const target = event.target as HTMLElement | null;
		const typing = target?.tagName === "INPUT" || target?.tagName === "TEXTAREA";

		if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
			if (!editingField) return;

			event.preventDefault();
			(document.getElementById("issue-edit-form") as HTMLFormElement | null)?.requestSubmit();

			return;
		}

		if (event.key === "Escape") {
			if (editingField) {
				event.preventDefault();
				discard();
			}

			return;
		}

	}
</script>

<svelte:window onkeydown={onKey} onpointerdown={onPointerDown} />


<svelte:head>
	<title>{issue ? `${issue.reference} · ${issue.title}` : "Issue"} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div
		class="flex h-11 flex-none items-center gap-2 border-b border-line-default pr-2.5 pl-2.5"
	>
		<Button variant="outline" size="icon-sm" href={at("/issues")} aria-label="Back to the list">
			<ChevronLeft aria-hidden="true" />
		</Button>

		<div class="flex min-w-0 flex-1 items-center gap-1.75">
			<a
				href={at("/issues")}
				class="text-md whitespace-nowrap text-ink-600 motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
			>
				Issues
			</a>
			{#if issue}
				{#if issue.projectName}
					<span class="text-sm text-text-disabled" aria-hidden="true">/</span>
					<a
						href={at(`/projects/${issue.projectId}`)}
						class="hidden text-md whitespace-nowrap text-ink-600 motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring sm:inline"
					>
						{issue.projectName}
					</a>
				{/if}
				<span class="text-sm text-text-disabled" aria-hidden="true">/</span>
				<span class="font-mono text-sm tracking-wide text-ink-900">{issue.reference}</span>
				<Button
					variant="ghost"
					size="icon-xs"
					aria-label="Copy the identifier"
					onclick={() => copy(issue.reference, `Copied ${issue.reference}`)}
				>
					<Copy aria-hidden="true" />
				</Button>
			{/if}
		</div>

		{#if issue && position >= 0}
			<span class="hidden font-mono text-xs text-muted-foreground sm:inline">
				{position + 1} of {order.length}
			</span>
			<div class="flex gap-0.5">
				<Button
					variant="outline"
					size="icon-sm"
					aria-label="Previous issue"
					disabled={!previous}
					href={previous ? at(`/issues/${previous.reference}`) : undefined}
				>
					<ChevronUp aria-hidden="true" />
				</Button>
				<Button
					variant="outline"
					size="icon-sm"
					aria-label="Next issue"
					disabled={!next}
					href={next ? at(`/issues/${next.reference}`) : undefined}
				>
					<ChevronDown aria-hidden="true" />
				</Button>
			</div>
		{/if}

		{#if issue}
			<Button
				variant="outline"
				size="icon-sm"
				aria-label={following ? "Stop following this issue" : "Follow this issue"}
				aria-pressed={following}
				class={following ? "bg-accent text-ink-900" : ""}
				disabled={followWorking}
				onclick={toggleFollow}
			>
				{#if following}
					<BellRing aria-hidden="true" />
				{:else}
					<BellOff aria-hidden="true" />
				{/if}
			</Button>

			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="outline" size="icon-sm" aria-label="More on this issue">
							<Ellipsis aria-hidden="true" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="w-53">
					<DropdownMenu.Item
						onSelect={() => copy(`${page.url.origin}${at(`/issues/${issue.reference}`)}`, `Copied a link to ${issue.reference}`)}
					>
						<Link2 aria-hidden="true" />
						Copy link
					</DropdownMenu.Item>
					{#if canEdit}
						<DropdownMenu.Item disabled={working} onSelect={duplicate}>
							<Copy aria-hidden="true" />
							Duplicate issue
						</DropdownMenu.Item>
						<DropdownMenu.Item onSelect={() => (parentPicking = true)}>
							<CornerDownLeft aria-hidden="true" />
							Convert to sub-issue
						</DropdownMenu.Item>
						<DropdownMenu.Separator />
						<DropdownMenu.Item disabled={working} onSelect={() => setStatus("archived")}>
							<Archive aria-hidden="true" />
							Archive issue
						</DropdownMenu.Item>
						<DropdownMenu.Item
							variant="destructive"
							disabled={working}
							onSelect={() => setStatus("pending_deletion")}
						>
							<Trash2 aria-hidden="true" />
							Delete issue
						</DropdownMenu.Item>
					{/if}
				</DropdownMenu.Content>
			</DropdownMenu.Root>

			<Button variant="outline" size="icon-sm" href={at("/issues")} aria-label="Close this issue">
				<X aria-hidden="true" />
			</Button>
		{/if}
	</div>

	{#snippet banner(glyph: Snippet, line: string, detail: string, action?: Snippet)}
		<div
			class="flex min-h-8 flex-none flex-wrap items-center gap-2 border-b border-line-strong bg-paper-2 px-3.5 py-1.5"
		>
			{@render glyph()}
			<span class="text-sm text-ink-600">{line}</span>
			<span class="text-sm text-muted-foreground">{detail}</span>
			<div class="flex-1"></div>
			{@render action?.()}
		</div>
	{/snippet}

	{#if issue && ready}
		{#if closedIssue && issue.status === "active"}
			{#snippet closedGlyph()}
				<StatusIcon category={issue.state.category} decorative />
			{/snippet}
			{#snippet reopenAction()}
				{#if canEdit && reopenState}
					<Button variant="ghost" size="sm" disabled={working} onclick={reopen}>Reopen</Button>
				{/if}
			{/snippet}
			{@render banner(
				closedGlyph,
				`${issue.state.name} since ${issue.completedAt ? onDate(issue.completedAt, data.workspace.timezone) : when(issue.stateEnteredAt)}`,
				"· reopening keeps the history",
				reopenAction
			)}
		{/if}

		{#if issue.status !== "active"}
			{#snippet archivedGlyph()}
				<Archive class="size-icon-row text-muted-foreground" aria-hidden="true" />
			{/snippet}
			{#snippet restoreAction()}
				{#if role !== "viewer"}
					<Button variant="ghost" size="sm" disabled={working} onclick={() => setStatus("active")}>
						Restore
					</Button>
				{/if}
			{/snippet}
			{@render banner(
				archivedGlyph,
				issue.status === "archived"
					? `Archived ${issue.archivedAt ? onDate(issue.archivedAt, data.workspace.timezone) : ""}.`
					: "This issue is on its way out.",
				issue.status === "archived"
					? "Archived issues are read-only."
					: "It is removed for good after 30 days.",
				restoreAction
			)}
		{/if}

		{#if role === "viewer"}
			{#snippet readOnlyGlyph()}
				<Info class="size-icon-row text-muted-foreground" aria-hidden="true" />
			{/snippet}
			{@render banner(
				readOnlyGlyph,
				`You have read access to ${issue.teamKey}.`,
				"Properties, comments and attachments are locked."
			)}
		{/if}

		{#if issue.blocked}
			{#snippet blockedGlyph()}
				<Link2 class="size-icon-row text-muted-foreground" aria-hidden="true" />
			{/snippet}
			{@render banner(
				blockedGlyph,
				"Something this issue is blocked by is still open.",
				"It cannot be started until that clears."
			)}
		{/if}
	{/if}

	{#if detail.kind === "loading"}
		<div class="flex min-h-0 flex-1">
			<div class="min-w-0 flex-1 px-8 py-6">
				<div class="mx-auto flex max-w-192 flex-col gap-3.5" aria-busy="true">
					<span class="h-6 w-3/5 animate-breathe rounded-xs bg-paper-3"></span>
					<span class="h-3 w-1/3 animate-breathe rounded-xs bg-paper-2"></span>
					<span class="my-1.5 h-px bg-line-subtle"></span>
					{#each ["w-full", "w-11/12", "w-3/4", "w-1/2"] as width (width)}
						<span class="h-3 {width} animate-breathe rounded-xs bg-paper-2"></span>
					{/each}
				</div>
			</div>
			<aside class="hidden w-75 flex-none flex-col gap-3 border-l border-line-default p-3.5 lg:flex">
				{#each [1, 2, 3, 4, 5, 6] as row (row)}
					<span class="flex items-center gap-2.5">
						<span class="h-2.5 w-15 animate-breathe rounded-xs bg-paper-2"></span>
						<span class="h-3 flex-1 animate-breathe rounded-xs bg-paper-3"></span>
					</span>
				{/each}
			</aside>
		</div>
	{:else if detail.kind === "not_found"}
		<div class="flex flex-1 items-center justify-center p-10">
			<div class="flex max-w-95 flex-col items-center gap-2.5 text-center">
				<CircleX class="size-4 text-muted-foreground" aria-hidden="true" />
				<span class="font-mono text-xs tracking-eyebrow text-ink-600 uppercase">
					{page.params.reference} is no longer available
				</span>
				<p class="text-md leading-normal text-muted-foreground text-pretty">
					It was deleted, or it belongs to a team you are not on. Links to it keep resolving here.
				</p>
				<div class="mt-1 flex gap-2">
					<Button size="sm" href={at("/issues")}>Back to issues</Button>
				</div>
			</div>
		</div>
	{:else if detail.kind === "unavailable" || !issue || !ready}
		<div class="flex flex-1 items-start justify-center p-10">
			<Alert.Root variant="destructive" class="max-w-120">
				<CircleX aria-hidden="true" />
				<Alert.Title>We could not load this issue</Alert.Title>
				<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
			</Alert.Root>
		</div>
	{:else}
		<div class="flex min-h-0 flex-1 flex-col overflow-y-auto lg:flex-row lg:overflow-hidden">
			<div class="min-w-0 flex-1 lg:overflow-auto">
				<div
					class="mx-auto flex max-w-192 flex-col gap-6.5 px-4 pt-5 pb-16 sm:px-8 pb-[calc(--spacing(16)+env(safe-area-inset-bottom))]"
				>
					{#if labelFailure}
						<Alert.Root variant="destructive">
							<CircleX aria-hidden="true" />
							<Alert.Title>That did not work</Alert.Title>
							<Alert.Description>{labelFailureMessage(labelFailure)}</Alert.Description>
						</Alert.Root>
					{/if}

					{#if delegationFailure}
						<Alert.Root variant="destructive">
							<CircleX aria-hidden="true" />
							<Alert.Title>That did not work</Alert.Title>
							<Alert.Description>
								{delegationFailureMessage(delegationFailure)}
							</Alert.Description>
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

					<form
						id="issue-edit-form"
						method="POST"
						use:enhance
						class="flex flex-col gap-2.25"
					>
						{#if issue.parentReference}
							<a
								href={at(`/issues/${issue.parentReference}`)}
								class="-ml-1.25 inline-flex h-5.5 w-max max-w-full items-center gap-1.75 rounded-chip border border-line-default pr-1.75 pl-1.25 motion-control hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
							>
								<CornerDownLeft class="size-3.25 text-muted-foreground" aria-hidden="true" />
								<span class="font-mono text-xs text-muted-foreground">{issue.parentReference}</span>
								<span class="truncate text-sm text-ink-600">Parent issue</span>
							</a>
						{/if}

						{#if editingField === "title"}
							<div class="flex flex-col gap-2.5">
								<Form.Field {form} name="title">
									<Form.Control>
										{#snippet children({ props })}
											<Form.Label class="sr-only">Title</Form.Label>
											<input
												{...props}
												bind:this={titleField}
												bind:value={$formData.title}
												disabled={$submitting}
												class="-mx-2.25 w-[calc(100%+1.125rem)] border-b-2 border-cyan-500 bg-transparent px-2.25 py-1.25 text-2xl leading-tight font-medium tracking-title text-ink-900 outline-none"
											/>
										{/snippet}
									</Form.Control>
									<Form.FieldErrors />
								</Form.Field>
								<div class="flex items-center gap-2">
									<Button type="submit" size="sm" disabled={$submitting}>
										{$submitting ? "Saving" : "Save title"}
									</Button>
									<Button
										type="button"
										variant="ghost"
										size="sm"
										disabled={$submitting}
										onclick={discard}
									>
										Cancel
									</Button>
									<span class="flex items-center gap-1.5 text-xs text-muted-foreground">
										<Kbd keys="⌘ ↵" /> save
									</span>
								</div>
							</div>
						{:else}
							<div class="flex items-start gap-1">
								<button
									type="button"
									disabled={!canEdit}
									onclick={() => startEditing("title")}
									title={canEdit ? "Click to rename" : ""}
									class="-mx-2.25 -my-1.25 min-w-0 rounded-md px-2.25 py-1.25 text-left motion-control enabled:cursor-text enabled:hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
								>
									<h1
										class="text-2xl leading-tight font-medium tracking-title text-ink-900 text-pretty {closedIssue
											? 'line-through decoration-line-strong'
											: ''}"
									>
										{issue.title}
									</h1>
								</button>
								<Button
									variant="ghost"
									size="icon-xs"
									aria-label="Copy the title"
									class="mt-1"
									onclick={() => copy(issue.title, "Copied the title")}
								>
									<Copy aria-hidden="true" />
								</Button>
							</div>
						{/if}

						<div class="flex flex-wrap items-center gap-2">
							<Avatar.Root size="xs">
								<Avatar.Fallback>{initials(reporter)}</Avatar.Fallback>
							</Avatar.Root>
							<span class="text-sm text-muted-foreground">{reporter} opened this</span>
							<span class="font-mono text-xs text-muted-foreground">
								<time datetime={issue.createdAt}>
									{onDate(issue.createdAt, data.workspace.timezone)}
								</time>
							</span>
							<span class="text-text-disabled" aria-hidden="true">·</span>
							<span class="font-mono text-xs text-muted-foreground">
								{commentTotal}
								{commentTotal === 1 ? "comment" : "comments"}
							</span>
						</div>

						<div class="mt-3.5 flex flex-col gap-3">
							<div class="flex items-center gap-2.5">
								<h2 class="min-w-0 flex-1">
									<Eyebrow rule class="text-ink-600">Description</Eyebrow>
								</h2>
								{#if canEdit && editingField !== "description"}
									<Button variant="ghost" size="sm" onclick={() => startEditing("description")}>
										<Pencil aria-hidden="true" />
										Edit
									</Button>
								{/if}
							</div>

							{#if editingField === "description"}
								<div data-editing-region class="flex flex-col gap-3">
									<Form.Field {form} name="description">
										<Form.Control>
											{#snippet children({ props })}
												<Form.Label class="sr-only">Description</Form.Label>
												<div
													class="flex flex-col overflow-hidden rounded-md border border-line-strong"
												>
													<div
														class="flex h-7.5 items-center gap-0.5 border-b border-line-subtle bg-paper-0 px-1.25"
													>
														<span class="flex-1"></span>
														<span class="font-mono text-2xs text-muted-foreground">Markdown</span>
													</div>
													<textarea
														{...props}
														bind:this={descriptionField}
														bind:value={$formData.description}
														disabled={$submitting}
														rows={8}
														class="min-h-47 w-full resize-y bg-paper-0 px-3 py-2.75 text-base leading-normal text-ink-900 outline-none"
													></textarea>
												</div>
											{/snippet}
										</Form.Control>
										<Form.FieldErrors />
									</Form.Field>

									<UploadList
										uploads={attachmentPreview?.bodyUploads ?? bodyUploads}
										oncancel={cancelUpload}
										onretry={(taskId) => retryUpload("body", taskId)}
										ondismiss={(taskId) => dismissUpload("body", taskId)}
									/>
								</div>
							{:else if issue.description.trim()}
								<button
									type="button"
									disabled={!canEdit}
									onclick={() => startEditing("description")}
									class="-mx-2.25 -my-1.75 rounded-md px-2.25 py-1.75 text-left motion-control enabled:cursor-text enabled:hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
								>
									<Markdown source={issue.description} />
								</button>
							{:else}
								<button
									type="button"
									disabled={!canEdit}
									onclick={() => startEditing("description")}
									class="-mx-2.25 -my-1.75 rounded-md px-2.25 py-1.75 text-left text-md text-muted-foreground motion-control enabled:cursor-text enabled:hover:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
								>
									{canEdit ? "Add a description" : "No description."}
								</button>
							{/if}
						</div>
					</form>

					{#if changed.length > 0}
						<section class="flex min-w-0 flex-col gap-1.5">
							<div class="flex items-center gap-2.5">
								<h2 class="min-w-0 flex-1">
									<Eyebrow rule class="text-ink-600">What changed</Eyebrow>
								</h2>
							</div>
							<p class="font-mono text-xs text-muted-foreground">
								{changeStatLine(changeTotals(changed))}
							</p>
							<ul class="flex min-w-0 flex-col">
								{#each changed as change (change.repository)}
									<RepoChange
										{change}
										links={codeLinks}
										diff={{ kind: "idle" }}
										download={`/v1/workspaces/${data.workspace.id}/executions/${change.executionId}/artifacts/${change.diffArtifactId}/content`}
									/>
								{/each}
							</ul>
						</section>
					{/if}

					{#if runs.length > 0}
						<section class="flex min-w-0 flex-col gap-1.5">
							<div class="flex items-center gap-2.5">
								<h2 class="min-w-0 flex-1">
									<Eyebrow rule class="text-ink-600">Runs</Eyebrow>
								</h2>
							</div>
							<ul class="flex min-w-0 flex-col">
								{#each runs as run (run.id)}
									<li class="min-w-0">
										<RunRow
											execution={run}
											href={workspacePath(data.workspace.slug, `/executions/${run.id}`)}
											timezone={data.workspace.timezone}
										/>
									</li>
								{/each}
							</ul>
						</section>
					{/if}

					{#if questions.length > 0}
						<section class="flex min-w-0 flex-col gap-1.5">
							<div class="flex items-center gap-2.5">
								<h2 class="min-w-0 flex-1">
									<Eyebrow rule class="text-ink-600">Questions</Eyebrow>
								</h2>
							</div>
							{#if questionFailed}
								<Alert.Root variant="destructive">
									<CircleX aria-hidden="true" />
									<Alert.Title>That did not stick</Alert.Title>
									<Alert.Description>The answer was not recorded. Reload the issue and try again.</Alert.Description>
								</Alert.Root>
							{/if}
							<QuestionList
								{questions}
								timezone={data.workspace.timezone}
								canAnswer={canEdit}
								{working}
								onanswer={answerQuestion}
								ondismiss={dismissQuestion}
							/>
						</section>
					{/if}

					<section class="flex flex-col gap-1.5">
						<div class="flex items-center gap-2.5">
							<h2 class="min-w-0 flex-1">
								<Eyebrow rule class="text-ink-600">
									Sub-issues
								</Eyebrow>
							</h2>
							{#if ready.children.length > 0}
								<span class="inline-flex items-center gap-1.75">
									<span class="font-mono text-xs text-muted-foreground">
										{ready.childProgress.complete}/{totalIssues(ready.childProgress)}
									</span>
									<ProgressBar progress={ready.childProgress} label={false} class="w-14" />
								</span>
							{/if}
							{#if canEdit}
								<Button
									variant="ghost"
									size="icon-xs"
									aria-label="Add a sub-issue"
									onclick={() => {
										childPrefill = undefined;
										addingChild = true;
									}}
								>
									<Plus aria-hidden="true" />
								</Button>
							{/if}
						</div>
						<IssueChildren
							children={ready.children}
							{at}
							states={canEdit ? ready.states : []}
							now={data.now}
							timezone={data.workspace.timezone}
							nameOf={nameOf}
							{working}
							onstate={moveChild}
						/>
					</section>

					<section class="flex flex-col gap-1.5">
						<div class="flex items-center gap-2.5">
							<h2 class="min-w-0 flex-1">
								<Eyebrow rule class="text-ink-600">
									Links
								</Eyebrow>
							</h2>
						</div>
						<IssueRelations
							{issue}
							groups={ready.relations}
							candidates={ready.candidates}
							{at}
							working={working || !canEdit}
							onadd={addRelation}
							onremove={removeRelation}
						/>
					</section>

					<section class="flex flex-col gap-1.5">
						<div class="flex items-center gap-2.5">
							<h2 class="min-w-0 flex-1">
								<Eyebrow rule class="text-ink-600">
									Attachments
								</Eyebrow>
							</h2>
							{#if canEdit}
								<span data-editing-region class="contents">
									<AttachmentPicker
										disabled={working}
										label="Attach"
										onfiles={(files) => begin("body", files)}
									/>
								</span>
							{/if}
						</div>

						{#if attachmentFailure}
							<Alert.Root variant="destructive">
								<CircleX aria-hidden="true" />
								<Alert.Title>That did not stick</Alert.Title>
								<Alert.Description>{attachmentFailureMessage(attachmentFailure)}</Alert.Description>
							</Alert.Root>
						{/if}

						<AttachmentList
							panel={attachments}
							working={working || !canEdit}
							onremove={removeAttachment}
						/>

						{#if editingField !== "description"}
							<UploadList
								uploads={attachmentPreview?.bodyUploads ?? bodyUploads}
								oncancel={cancelUpload}
								onretry={(taskId) => retryUpload("body", taskId)}
								ondismiss={(taskId) => dismissUpload("body", taskId)}
							/>
						{/if}
				</section>

					<section class="flex flex-col gap-0.5">
						<div class="mb-1.5 flex items-center gap-2.5">
							<h2 class="min-w-0 flex-1">
								<Eyebrow rule class="text-ink-600">
									Linked changes
								</Eyebrow>
							</h2>
						</div>

						{#if codeLinkFailure}
							<Alert.Root variant="destructive">
								<CircleX aria-hidden="true" />
								<Alert.Title>That did not stick</Alert.Title>
								<Alert.Description>{codeFailureMessage(codeLinkFailure)}</Alert.Description>
							</Alert.Root>
						{/if}

						<CodeLinkPanel
							links={codeLinks}
							busy={unlinking}
							onunlink={canEdit ? unlinkCode : undefined}
						/>

						{#if shipping.releases.length > 0 || shipping.deployments.length > 0}
							<div class="flex flex-col gap-1 pt-1">
								{#if shipping.releases.length > 0}
									<p class="text-sm text-ink-900">
										Shipped in
										{#each shipping.releases as release, index (release.id)}{#if index > 0}, {/if}{#if release.url}<a
												href={release.url}
												target="_blank"
												rel="noreferrer"
												class="underline underline-offset-2">{releaseLabel(release)}</a
											>{:else}{releaseLabel(release)}{/if}{/each}
									</p>
								{/if}
								{#if shipping.deployments.length > 0}
									<ul class="flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
										{#each shipping.deployments as deployment (deployment.id)}
											<li class:text-success={deployment.state === "succeeded"}
												class:text-destructive={deployment.state === "failed"}>
												{deploymentLabel(deployment)}
											</li>
										{/each}
									</ul>
								{/if}
							</div>
						{/if}

						{#if mirrorConflicts.length > 0}
							<div class="flex flex-col gap-2 rounded-md border border-warning/40 p-3">
								<p class="text-sm text-ink-900">
									An edit here and one on the platform landed together. Whichever moved last was
									kept; the other is below so nothing is lost.
								</p>
								{#each mirrorConflicts as conflict (conflict.id)}
									<div class="flex flex-col gap-0.5">
										<p class="text-xs text-muted-foreground">
											{conflict.field} · {conflictSummary(conflict)}
										</p>
										<pre class="overflow-x-auto rounded-sm bg-muted p-2 text-xs whitespace-pre-wrap text-ink-900">{conflict.discarded}</pre>
									</div>
								{/each}
							</div>
						{/if}

						{#if canEdit}
							<div class="flex flex-wrap items-center gap-2 pt-1">
								<Button variant="secondary" onclick={copyBranchName} disabled={copyingBranch}>
									{branchCopied ? "Copied" : copyingBranch ? "Reading…" : "Copy branch name"}
								</Button>
								<Button
									variant="ghost"
									onclick={() => setAutomation(!automationSuppressed)}
									disabled={togglingAutomation}
								>
									{automationSuppressed ? "Resume automation" : "Stop moving this issue"}
								</Button>
							</div>

							{#if branchName}
								<p class="font-mono text-xs break-all text-muted-foreground">{branchName}</p>
							{/if}

							{#if automationSuppressed}
								<p class="text-xs text-muted-foreground">
									Source control still records what happens to the code here; it just does not
									move this issue.
								</p>
							{/if}
						{/if}
					</section>

					<section class="flex flex-col gap-0.5">
						<div class="mb-1.5 flex items-center gap-2.5">
							<h2 class="min-w-0 flex-1">
								<Eyebrow rule class="text-ink-600">
									Activity
								</Eyebrow>
							</h2>
							<div class="inline-flex gap-0.5 rounded-sm border border-line-default p-0.5">
								{#each [{ key: "all", label: "All" }, { key: "comments", label: "Comments" }] as choice (choice.key)}
									<button
										type="button"
										aria-pressed={shown === choice.key}
										onclick={() => (shown = choice.key as "all" | "comments")}
										class="h-5 cursor-pointer rounded-xs px-2 font-mono text-2xs tracking-eyebrow uppercase motion-control {shown ===
										choice.key
											? 'bg-primary text-primary-foreground'
											: 'text-ink-600 hover:bg-accent'}"
									>
										{choice.label}
									</button>
								{/each}
							</div>
						</div>

						<CommentThreadView
							{thread}
							events={shown === "all" ? events : []}
							uploads={shownCommentUploads}
							onfiles={(files) => begin("comment", files)}
							oncancelupload={cancelUpload}
							onretryupload={(taskId) => retryUpload("comment", taskId)}
							ondismissupload={(taskId) => dismissUpload("comment", taskId)}
							members={ready.members}
							teams={data.teams ?? []}
							accountId={data.member.id}
							{when}
							{canEdit}
							lockedLine={issue.status === "archived"
								? "Archived issues cannot be commented on. Restore it first."
								: `You have read access to ${issue.teamKey}. Ask an admin to comment.`}
							working={working || Boolean(commentPreview?.working)}
							failure={commentFailure ?? commentPreview?.failure ?? null}
							unreachable={commentPreview?.unreachable ?? unreachable}
							onpost={comment}
							onedit={editComment}
							onremove={removeComment}
							onreact={react}
							onmore={loadEarlier}
						/>
					</section>
				</div>
			</div>

			<aside
				class="w-full flex-none border-t border-line-default px-3.5 pt-3.5 pb-6 lg:w-75 lg:overflow-auto lg:border-t-0 lg:border-l"
			>
				<div class="flex flex-col gap-0.75">
					<IssueField
						label="Status"
						placeholder="Move to"
						editable={canEdit}
						options={ready.states.map((state) => ({
							value: state.id,
							label: state.name,
							checked: state.id === issue.state.id,
						}))}
						onpick={move}
					>
						{#snippet glyph()}
							<StatusIcon category={issue.state.category} decorative />
						{/snippet}
						{#snippet value()}
							<span class="min-w-0 flex-1 truncate">{issue.state.name}</span>
						{/snippet}
					</IssueField>

					<IssueField
						label="Priority"
						placeholder="Set priority"
						editable={canEdit}
						options={priorities.map((choice) => ({
							value: choice.value,
							label: choice.label,
							checked: choice.value === issue.priority,
						}))}
						onpick={(picked) => setPriority(picked as IssuePriority)}
					>
						{#snippet glyph()}
							<PriorityIcon priority={issue.priority} />
						{/snippet}
						{#snippet value()}
							<span class="min-w-0 flex-1 truncate">{priorityLabel(issue.priority)}</span>
						{/snippet}
					</IssueField>

					<IssueField
						label="Assignee"
						placeholder="Assign to"
						editable={canEdit}
						options={[
							{ value: "", label: "Unassigned", checked: !issue.assigneeAccountId },
							...people.map((member) => ({
								value: member.accountId,
								label: member.displayName || member.email || member.accountId,
								checked: member.accountId === issue.assigneeAccountId,
							})),
						]}
						onpick={setAssignee}
					>
						{#snippet glyph()}
							{#if assigneeName}
								<Avatar.Root size="xs">
									<Avatar.Fallback>{initials(assigneeName)}</Avatar.Fallback>
								</Avatar.Root>
							{:else}
								<UserRound class="size-icon-row text-muted-foreground" aria-hidden="true" />
							{/if}
						{/snippet}
						{#snippet value()}
							<span
								class="min-w-0 flex-1 truncate {assigneeName ? '' : 'text-muted-foreground'}"
							>
								{assigneeName || "Unassigned"}
							</span>
						{/snippet}
					</IssueField>

					<DelegationField
						panel={delegation}
						editable={canEdit}
						{assigned}
						working={working || Boolean(delegationPreview?.working)}
						timezone={data.workspace.timezone}
						ondelegate={() => (delegating = true)}
						onrecall={recall}
					/>

					<IssueField
						label="Labels"
						placeholder="Search labels"
						editable={canEdit && available.length > 0}
						closeOnPick={false}
						empty="No labels apply to this team"
						options={available.map((label) => ({
							value: label.id,
							label: label.name,
							checked: labels.some((chosen) => chosen.id === label.id),
						}))}
						onpick={(labelId) => {
							const chosen = available.find((candidate) => candidate.id === labelId);
							if (chosen) submit(toggled(labels, chosen));
						}}
					>
						{#snippet glyph()}
							<Tags class="size-icon-row text-muted-foreground" aria-hidden="true" />
						{/snippet}
						{#snippet value()}
							<span class="flex min-w-0 flex-1 flex-wrap gap-1.5">
								{#each labels as label (label.id)}
									<Tag name={label.name} color={label.color} />
								{/each}
								{#if labels.length === 0}
									<span class="text-muted-foreground">
										{canEdit ? "Add labels" : "None"}
									</span>
								{/if}
							</span>
						{/snippet}
					</IssueField>

					<IssueField
						label="Project"
						placeholder="Move to project"
						editable={canEdit}
						empty="No projects in this workspace yet"
						options={[
							{ value: "", label: "No project", checked: !issue.projectId },
							...ready.projects.map((project) => ({
								value: project.id,
								label: project.name,
								checked: project.id === issue.projectId,
							})),
						]}
						onpick={setProject}
					>
						{#snippet glyph()}
							<Folder class="size-icon-row text-muted-foreground" aria-hidden="true" />
						{/snippet}
						{#snippet value()}
							<span
								class="min-w-0 flex-1 truncate {issue.projectName ? '' : 'text-muted-foreground'}"
							>
								{issue.projectName || "No project"}
							</span>
						{/snippet}
					</IssueField>

					<IssueField
						label="Cycle"
						placeholder="Move to cycle"
						editable={canEdit}
						empty="This team runs no cycles"
						options={[
							{ value: "", label: "No cycle", checked: !issue.cycleId },
							...ready.cycles.map((cycle) => ({
								value: cycle.id,
								label: `${cycle.name} · ${cycleWindow(cycle.startsOn, cycle.endsOn)}`,
								checked: cycle.id === issue.cycleId,
							})),
						]}
						onpick={setCycle}
					>
						{#snippet glyph()}
							<Layers class="size-icon-row text-muted-foreground" aria-hidden="true" />
						{/snippet}
						{#snippet value()}
							<span
								class="min-w-0 flex-1 truncate {issue.cycleNumber ? '' : 'text-muted-foreground'}"
							>
								{issue.cycleNumber ? `Cycle ${issue.cycleNumber}` : "No cycle"}
							</span>
						{/snippet}
					</IssueField>

					<div class="relative flex min-h-7 items-center gap-1.5">
						<span
							class="w-19.5 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
						>
							Due
						</span>
						{#if canEdit}
							<Popover.Root bind:open={pickingDue}>
								<Popover.Trigger>
									{#snippet child({ props })}
										<button
											{...props}
											type="button"
											aria-label="Due date: change"
											class="-ml-1.75 flex min-h-6 min-w-0 flex-1 cursor-pointer items-center gap-1.75 rounded-sm px-1.75 py-0.5 text-left text-md motion-control hover:bg-accent aria-expanded:bg-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
										>
											<CalendarDays class="size-icon-row text-muted-foreground" aria-hidden="true" />
											<span
												class="truncate {issue.dueOn
													? overdue(issue.dueOn, data.now)
														? 'text-priority-urgent'
														: 'text-ink-900'
													: 'text-muted-foreground'}"
											>
												{issue.dueOn
													? dueLabel(issue.dueOn, data.now, data.workspace.timezone)
													: "No due date"}
											</span>
										</button>
									{/snippet}
								</Popover.Trigger>
								<Popover.Content align="start" class="w-auto p-0">
									<Calendar
										type="single"
										value={due}
										onValueChange={(picked) => {
											pickingDue = false;
											setDue(picked ? picked.toString() : "");
										}}
									/>
									{#if issue.dueOn}
										<div class="border-t border-line-subtle p-1.5">
											<Button
												variant="ghost"
												size="sm"
												class="w-full"
												onclick={() => {
													pickingDue = false;
													setDue("");
												}}
											>
												Clear the due date
											</Button>
										</div>
									{/if}
								</Popover.Content>
							</Popover.Root>
						{:else}
							<span class="flex min-h-6 min-w-0 flex-1 items-center gap-1.75 text-md">
								<CalendarDays class="size-icon-row text-muted-foreground" aria-hidden="true" />
								<span class="truncate {issue.dueOn ? 'text-ink-900' : 'text-muted-foreground'}">
									{issue.dueOn
										? dueLabel(issue.dueOn, data.now, data.workspace.timezone)
										: "No due date"}
								</span>
							</span>
						{/if}
					</div>

					<IssueField
						label="Estimate"
						placeholder="Set an estimate"
						editable={canEdit}
						options={[
							{ value: "", label: "No estimate", checked: !issue.estimate },
							...[1, 2, 3, 5, 8].map((points) => ({
								value: String(points),
								label: `${points} ${points === 1 ? "point" : "points"}`,
								checked: issue.estimate === points,
							})),
						]}
						onpick={setEstimate}
					>
						{#snippet glyph()}
							<Target class="size-icon-row text-muted-foreground" aria-hidden="true" />
						{/snippet}
						{#snippet value()}
							<span class="min-w-0 flex-1 truncate {issue.estimate ? '' : 'text-muted-foreground'}">
								{issue.estimate
									? `${issue.estimate} ${issue.estimate === 1 ? "point" : "points"}`
									: "No estimate"}
							</span>
						{/snippet}
					</IssueField>

					<IssueField
						label="Parent"
						placeholder="Search issues"
						editable={canEdit}
						bind:open={parentPicking}
						options={[
							{ value: "", label: "No parent", checked: !issue.parentId },
							...ready.candidates
								.filter((candidate) => candidate.id !== issue.id)
								.slice(0, 60)
								.map((candidate) => ({
									value: candidate.id,
									label: `${candidate.reference} ${candidate.title}`,
									checked: candidate.id === issue.parentId,
								})),
						]}
						onpick={(parentId) => setParent(parentId === "" ? null : parentId)}
					>
						{#snippet glyph()}
							<CornerDownLeft class="size-icon-row text-muted-foreground" aria-hidden="true" />
						{/snippet}
						{#snippet value()}
							<span
								class="min-w-0 flex-1 truncate {issue.parentReference ? '' : 'text-muted-foreground'}"
							>
								{issue.parentReference || "No parent"}
							</span>
						{/snippet}
					</IssueField>

					<IssueField
						label="Team"
						placeholder="Move to team"
						editable={canEdit}
						options={teams.map((team) => ({
							value: team.id,
							label: `${team.key} · ${team.name}`,
							checked: team.id === issue.teamId,
						}))}
						onpick={(teamId) => moveToTeam(teamId, false)}
					>
						{#snippet glyph()}
							<Users class="size-icon-row text-muted-foreground" aria-hidden="true" />
						{/snippet}
						{#snippet value()}
							<span class="min-w-0 flex-1 truncate">{issue.teamKey}</span>
						{/snippet}
					</IssueField>

					<span class="my-2.5 h-px bg-line-subtle"></span>

					<div class="flex min-h-7 items-center gap-2">
						<span
							class="w-19.5 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
						>
							Watching
						</span>
						{#if watchers.length > 0}
							<span class="flex min-w-0 flex-1 -space-x-1.5">
								{#each watchers.slice(0, 5) as watcher (watcher.accountId)}
									<Avatar.Root size="xs" class="ring-1 ring-card" title={watcher.name}>
										<Avatar.Fallback>{initials(watcher.name)}</Avatar.Fallback>
									</Avatar.Root>
								{/each}
								{#if watchers.length > 5}
									<span
										class="inline-flex size-icon-row items-center justify-center rounded-full bg-paper-2 font-mono text-2xs text-muted-foreground ring-1 ring-card"
									>
										+{watchers.length - 5}
									</span>
								{/if}
							</span>
						{:else}
							<span class="flex-1 truncate text-md text-muted-foreground">Nobody yet</span>
						{/if}
						<Button variant="ghost" size="sm" disabled={followWorking} onclick={toggleFollow}>
							{following ? "Unsubscribe" : "Subscribe"}
						</Button>
					</div>

					<span class="my-2.5 h-px bg-line-subtle"></span>

					<div class="flex min-h-5.5 items-center gap-1.5">
						<span
							class="w-19.5 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
						>
							Created
						</span>
						<span class="font-mono text-xs text-muted-foreground">
							<time datetime={issue.createdAt}>{when(issue.createdAt)}</time>
						</span>
					</div>

					{#if lastActivity}
						<div class="flex min-h-5.5 items-center gap-1.5">
							<span
								class="w-19.5 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
							>
								Updated
							</span>
							<span class="font-mono text-xs text-muted-foreground">
								<time datetime={lastActivity}>{when(lastActivity)}</time>
							</span>
						</div>
					{/if}

					<div class="flex min-h-5.5 items-center gap-1.5">
						<span
							class="w-19.5 flex-none font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase"
						>
							Link
						</span>
						<button
							type="button"
							aria-label="Copy the link to {issue.reference}"
							title="Copy"
							onclick={() =>
								copy(`${page.url.origin}${at(`/issues/${issue.reference}`)}`, `Copied a link to ${issue.reference}`)}
							class="flex min-w-0 cursor-pointer items-center gap-1.5 font-mono text-xs text-muted-foreground motion-control hover:text-ink-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
						>
							<span class="min-w-0 truncate">{issueLink}</span>
							<Copy class="size-3 flex-none" aria-hidden="true" />
						</button>
					</div>
				</div>
			</aside>
		</div>

		{#if editingField && (dirty || $submitting)}
			<div
				data-editing-region
				class="flex h-10 flex-none items-center gap-2.25 border-t border-line-strong bg-paper-2 px-4"
			>
				{#if $submitting}
					<CircleDashed class="size-icon-row text-muted-foreground" aria-hidden="true" />
					<span class="text-sm text-ink-600">Saving</span>
				{:else if failure}
					<TriangleAlert class="size-icon-row text-destructive" aria-hidden="true" />
					<span class="text-sm text-destructive">Could not save</span>
					<span class="text-sm text-muted-foreground">· nothing was changed</span>
				{:else}
					<CircleDot class="size-icon-row text-muted-foreground" aria-hidden="true" />
					<span class="text-sm text-ink-600">Unsaved changes</span>
					<span class="text-sm text-muted-foreground">· ⌘↵ to save</span>
				{/if}
				<div class="flex-1"></div>
				{#if !$submitting}
					<Button variant="ghost" size="sm" onclick={discard}>Discard</Button>
					<Button type="submit" form="issue-edit-form" size="sm">
						{failure ? "Retry" : "Save"}
					</Button>
				{/if}
			</div>
		{/if}

		<div
			class="hidden h-7.5 flex-none items-center gap-4 border-t border-line-subtle px-3.5 lg:flex"
		>
			<span class="font-mono text-2xs tracking-eyebrow text-muted-foreground uppercase">
				{issue.reference}
			</span>
			<div class="flex-1"></div>
			<span class="flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys="E" /> edit description
			</span>
			<span class="flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys="⌘ ↵" /> save
			</span>
			<span class="flex items-center gap-1.5 text-xs text-muted-foreground">
				<Kbd keys="Esc" /> cancel
			</span>
		</div>
	{/if}
</div>

{#if ready && issue}
	<NewIssueDialog
		bind:open={addingChild}
		workspaceId={data.workspace.id}
		teams={data.teams ?? []}
		states={{ [issue.teamId]: ready.states }}
		members={ready.members.map((member) => ({
			accountId: member.accountId,
			displayName: member.displayName,
			kind: member.kind,
		}))}
		labels={ready.labels}
		projects={ready.projects}
		today={calendarDate(data.now, data.workspace.timezone)}
		now={data.now}
		prefill={childPrefill ?? { teamId: issue.teamId, projectId: issue.projectId ?? "" }}
		onsettled={settle}
	/>

	<DelegateDialog
		bind:open={delegating}
		workspaceId={data.workspace.id}
		issueId={issue.id}
		reference={issue.reference}
		{agents}
		ondelegated={(held) => {
			delegated = { kind: "held", delegation: held };
			invalidate(keys.page(page.route.id));
		}}
	/>

{/if}
