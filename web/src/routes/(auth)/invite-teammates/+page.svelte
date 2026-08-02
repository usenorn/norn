<script lang="ts">
	import { page } from "$app/state";
	import Check from "@lucide/svelte/icons/check";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Copy from "@lucide/svelte/icons/copy";
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import Link from "@lucide/svelte/icons/link";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import Users from "@lucide/svelte/icons/users";
	import X from "@lucide/svelte/icons/x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import {
		inviteRoles,
		isEmailAddress,
		parseAddresses,
		type Invite,
		type InviteRole,
		type InviteStatus,
	} from "$lib/workspace/invites";
	import { invitePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? invitePreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let text = $state("");
	let roles = $state<Record<string, InviteRole>>({});
	let removed = $state<string[]>([]);

	const workspace = $derived(data.auth.workspace);
	const emailConfigured = $derived(preview?.emailConfigured ?? true);
	const sending = $derived(preview?.sending ?? false);

	$effect(() => {
		const seed = preview?.text;
		if (seed !== undefined) text = seed;
	});

	const composed = $derived<Invite[]>(
		parseAddresses(text).map((email) => ({
			email,
			role: "Member",
			status: (isEmailAddress(email)
				? emailConfigured
					? "pending"
					: "link_only"
				: "invalid") as InviteStatus,
		}))
	);

	const rows = $derived<Invite[]>(
		(preview?.rows ?? composed)
			.filter((row) => !removed.includes(row.email))
			.map((row) => ({ ...row, role: roles[row.email] ?? row.role }))
	);

	const counts = $derived({
		total: rows.length,
		sendable: rows.filter((row) => row.status === "pending").length,
		linkOnly: rows.filter((row) => row.status === "link_only").length,
		attention: rows.filter(
			(row) => row.status === "invalid" || row.status === "existing_member"
		).length,
		sent: rows.filter((row) => row.status === "sent").length,
		failed: rows.filter((row) => row.status === "failed").length,
	});

	const allSent = $derived(counts.total > 0 && counts.sent === counts.total);
	const composing = $derived(!sending && counts.sent === 0 && counts.failed === 0);

	const title = $derived(
		allSent ? `Invited ${counts.sent} people to ${workspace}` : "Invite your team"
	);
	const lede = $derived(
		allSent
			? "Each one gets a link that's good for seven days."
			: counts.total === 0
				? "Paste a list of addresses. Roles are easy to change later."
				: "Roles are easy to change later."
	);

	const rowsLabel = $derived.by(() => {
		if (sending) return "Sending";
		if (counts.failed > 0) return "Results";
		if (allSent) return "Invited";
		if (counts.attention > 0) return `${counts.total} addresses`;
		return `${counts.total} people`;
	});

	const rowsRight = $derived.by(() => {
		if (sending) return { text: `${counts.sent} of ${counts.total} sent`, tone: "text-muted-foreground" };
		if (counts.failed > 0) return { text: `${counts.failed} failed`, tone: "text-destructive" };
		if (allSent) return { text: `${counts.sent} sent`, tone: "text-muted-foreground" };
		if (counts.attention > 0)
			return { text: `${counts.attention} need attention`, tone: "text-warning" };
		return null;
	});

	const notice = $derived.by(() => {
		if (!emailConfigured) {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Email delivery isn't configured",
				body: "This instance can't send anything until SMTP is set. Until then, copy each single-use link and share it yourself — they expire in seven days.",
				action: `Copy ${counts.linkOnly} invite links`,
			};
		}
		if (counts.failed > 0) {
			return {
				variant: "warning" as const,
				icon: TriangleAlert,
				title: `${counts.failed} invitations didn't send`,
				body: "Both addresses bounced at the provider. Everything else went out.",
				action: null,
			};
		}
		return null;
	});

	const cta = $derived.by(() => {
		if (sending) return "Sending";
		if (counts.failed > 0) return `Retry the ${counts.failed} that failed`;
		if (allSent) return "Continue";
		if (!emailConfigured && counts.linkOnly > 0) return `Copy ${counts.linkOnly} invite links`;
		if (counts.sendable > 0) return `Send ${counts.sendable} invitations`;
		return "Send invitations";
	});

	const secondary = $derived(sending ? null : allSent ? "Invite more" : "Skip for now");

	const footer = $derived(
		counts.attention > 0
			? "Invalid addresses are skipped, not sent. Fix them or remove them."
			: counts.total === 0
				? "You can invite people any time from Settings → Members."
				: ""
	);

	const rowGlyph = {
		pending: CircleDashed,
		invalid: CircleX,
		existing_member: CircleCheck,
		sent: Check,
		failed: TriangleAlert,
		link_only: Link,
	};
	const rowGlyphTone = {
		pending: "text-muted-foreground",
		invalid: "text-destructive",
		existing_member: "text-muted-foreground",
		sent: "text-success",
		failed: "text-warning",
		link_only: "text-muted-foreground",
	};
	const rowTone = {
		pending: "text-ink-900",
		invalid: "text-ink-900",
		existing_member: "text-muted-foreground",
		sent: "text-ink-600",
		failed: "text-ink-900",
		link_only: "text-ink-900",
	};
	const rowNote: Record<InviteStatus, string | null> = {
		pending: null,
		invalid: "not an email address",
		existing_member: "already a member",
		sent: "sent",
		failed: "mailbox not found",
		link_only: "link only",
	};
	const rowNoteTone = {
		pending: "text-muted-foreground",
		invalid: "text-destructive",
		existing_member: "text-muted-foreground",
		sent: "text-muted-foreground",
		failed: "text-warning",
		link_only: "text-muted-foreground",
	};

	const editable = (status: InviteStatus) => status === "pending" || status === "link_only";
	const removable = (status: InviteStatus) => editable(status) || status === "invalid";
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-140">
		<div class="flex flex-col gap-4 p-5 sm:p-6">
			<div class="flex flex-col gap-1.5">
				<h1 class="text-2xl font-medium tracking-title text-ink-900">{title}</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
			</div>

			{#if notice}
				{@const NoticeIcon = notice.icon}
				<Alert.Root variant={notice.variant}>
					<NoticeIcon aria-hidden="true" />
					<Alert.Title>{notice.title}</Alert.Title>
					<Alert.Description>{notice.body}</Alert.Description>
					{#if notice.action}
						<Alert.Action class="flex flex-wrap items-center gap-2">
							<Button variant="secondary" size="sm">
								<Copy aria-hidden="true" />
								{notice.action}
							</Button>
							<Button variant="ghost" size="sm">
								Configure SMTP
								<ExternalLink aria-hidden="true" />
							</Button>
						</Alert.Action>
					{/if}
				</Alert.Root>
			{/if}

			{#if composing}
				<div class="flex flex-col gap-1">
					<Label for="invite-addresses">Email addresses</Label>
					<Textarea
						id="invite-addresses"
						rows={3}
						placeholder={"jun@northwind.co, milo@northwind.co\nada@northwind.co"}
						bind:value={text}
					/>
					<p class="text-sm text-muted-foreground">
						Paste a list — commas, spaces or line breaks all work.
					</p>
				</div>
			{/if}

			{#if counts.total > 0}
				<div
					class="flex flex-col overflow-hidden rounded-lg border border-line-default bg-card"
				>
					<div
						class="flex h-7.5 items-center justify-between gap-2 border-b border-line-subtle px-2.5"
					>
						<Eyebrow class="text-ink-600">{rowsLabel}</Eyebrow>
						{#if rowsRight}
							<span class="font-mono text-xs {rowsRight.tone}">{rowsRight.text}</span>
						{/if}
					</div>
					<ul>
						{#each rows as row (row.email)}
							{@const RowGlyph = rowGlyph[row.status]}
							<li
								class="flex min-h-8.5 flex-wrap items-center gap-2 border-b border-line-subtle py-1 pr-2 pl-2.5 last:border-b-0"
							>
								<RowGlyph
									class="size-icon-row shrink-0 {rowGlyphTone[row.status]}"
									aria-hidden="true"
								/>
								<span
									class="min-w-0 flex-[1_1_160px] truncate font-mono text-sm {rowTone[row.status]}"
								>
									{row.email}
								</span>
								{#if rowNote[row.status]}
									<span class="shrink truncate text-sm {rowNoteTone[row.status]}">
										{rowNote[row.status]}
									</span>
								{/if}
								{#if editable(row.status) || removable(row.status)}
									<span class="ml-auto flex shrink-0 items-center gap-2">
										{#if editable(row.status)}
											<span class="w-[106px]">
												<Select.Root
													type="single"
													value={row.role}
													onValueChange={(value) => (roles[row.email] = value as InviteRole)}
												>
													<Select.Trigger size="sm" aria-label="Role for {row.email}">
														{row.role}
													</Select.Trigger>
													<Select.Content>
														{#each inviteRoles as role (role)}
															<Select.Item value={role} label={role}>{role}</Select.Item>
														{/each}
													</Select.Content>
												</Select.Root>
											</span>
										{/if}
										{#if removable(row.status)}
											<Button
												variant="ghost"
												size="icon-xs"
												aria-label="Remove {row.email}"
												onclick={() => (removed = [...removed, row.email])}
											>
												<X aria-hidden="true" />
											</Button>
										{/if}
									</span>
								{/if}
							</li>
						{/each}
					</ul>
				</div>
			{/if}

			{#if sending}
				<div class="flex items-center gap-2.5" aria-live="polite">
					<Progress
						value={counts.sent}
						max={counts.total}
						aria-label="Sending invitations"
						class="flex-1"
					/>
					<span class="font-mono text-xs text-muted-foreground tabular-nums">
						{counts.sent} of {counts.total}
					</span>
				</div>
			{/if}

			<div class="flex flex-wrap items-center gap-2">
				{#if allSent}
					<Button href="/import">{cta}</Button>
				{:else}
					<Button disabled={sending}>{cta}</Button>
				{/if}
				{#if secondary}
					<Button variant="ghost">{secondary}</Button>
				{/if}
			</div>

			<div class="flex items-start gap-2 border-t border-line-subtle pt-4">
				<Users class="mt-px size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					Norn doesn't charge per seat. Invite designers, support, anyone who reads the work — the
					price is the same at 5 people and at 500.
				</p>
			</div>
		</div>
	</div>

	{#if footer}
		<p class="max-w-120 text-center text-sm leading-normal text-muted-foreground text-pretty">
			{footer}
		</p>
	{/if}
</div>
