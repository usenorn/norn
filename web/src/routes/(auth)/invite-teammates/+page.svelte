<script lang="ts">
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
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
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import TeamKey from "$lib/components/norn/team-key.svelte";
	import { teamSummary } from "$lib/team/teams";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import { api } from "$lib/api";
	import { inviteSchema } from "$lib/workspace/invite-schema";
	import {
		counted,
		inviteFromResult,
		isEmailAddress,
		parseAddresses,
		type Invite,
		type InviteStatus,
	} from "$lib/workspace/invites";
	import { membershipRoles, roleLabels, type MembershipRole } from "$lib/workspace/members";
	import { invitePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "invite-form";
	const settleIntervalMs = 1500;
	const settleAttempts = 8;

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? invitePreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let roles = $state<Record<string, MembershipRole>>({});
	let teamIds = $state<Record<string, string[]>>({});
	let removed = $state<string[]>([]);
	let sent = $state<Invite[] | null>(null);
	let settling = $state(false);
	let copied = $state(false);
	let unavailable = $state(false);

	const workspace = $derived(data.target.name);

	const form = superForm(defaults(zod4(inviteSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(inviteSchema),
		resetForm: false,
		onUpdate: async ({ form: submitted }) => {
			if (!submitted.valid) return;

			unavailable = false;

			const requested = rows.map((row) => ({
				email: row.email,
				role: row.role,
				teamIds: row.teamIds,
			}));

			try {
				const { data: batch, error } = await api.POST(
					"/workspaces/{workspaceId}/invitations",
					{
						params: { path: { workspaceId: data.target.id } },
						body: { invitations: requested },
					}
				);

				if (error || !batch) {
					unavailable = true;

					return;
				}

				sent = batch.results.map((result, i) =>
					inviteFromResult(result, requested[i].role, requested[i].teamIds)
				);

				await settle();
			} catch {
				unavailable = true;
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const seed = preview?.text;
		if (seed !== undefined) formData.update((current) => ({ ...current, addresses: seed }), { taint: false });
	});

	async function settle() {
		if (!sent?.some((row) => row.status === "pending")) return;

		settling = true;

		for (let attempt = 0; attempt < settleAttempts; attempt++) {
			await new Promise((resolve) => setTimeout(resolve, settleIntervalMs));

			const { data: listed } = await api.GET("/workspaces/{workspaceId}/invitations", {
				params: { path: { workspaceId: data.target.id } },
			});

			if (!listed) break;

			const delivery = new Map(listed.map((invitation) => [invitation.id, invitation.delivery]));

			sent =
				sent?.map((row) => {
					const state = row.invitationId ? delivery.get(row.invitationId) : undefined;
					return state && state !== "pending" ? { ...row, status: state as InviteStatus } : row;
				}) ?? null;

			if (!sent?.some((row) => row.status === "pending")) break;
		}

		settling = false;
	}

	async function retryFailed() {
		const failed = rows.filter((row) => row.status === "failed" && row.invitationId);
		if (failed.length === 0) return;

		settling = true;

		for (const row of failed) {
			const { data: issued } = await api.POST(
				"/workspaces/{workspaceId}/invitations/{invitationId}/resend",
				{
					params: {
						path: { workspaceId: data.target.id, invitationId: row.invitationId! },
					},
				}
			);

			if (!issued) continue;

			sent =
				sent?.map((current) =>
					current.invitationId === row.invitationId
						? { ...current, status: issued.invitation.delivery as InviteStatus, url: issued.url }
						: current
				) ?? null;
		}

		settling = false;

		await settle();
	}

	async function copyLinks() {
		const links = rows.map((row) => row.url).filter((url): url is string => Boolean(url));
		if (links.length === 0) return;

		await navigator.clipboard.writeText(links.join("\n"));
		copied = true;
	}

	function inviteMore() {
		sent = null;
		removed = [];
		copied = false;
		formData.update((current) => ({ ...current, addresses: "" }), { taint: false });
	}

	async function remove(row: Invite) {
		if (!row.invitationId) {
			removed = [...removed, row.email];

			return;
		}

		settling = true;

		const { error } = await api.DELETE("/workspaces/{workspaceId}/invitations/{invitationId}", {
			params: { path: { workspaceId: data.target.id, invitationId: row.invitationId } },
		});

		settling = false;

		if (error) {
			unavailable = true;

			return;
		}

		removed = [...removed, row.email];
	}

	function statusOf(email: string): InviteStatus {
		if (!isEmailAddress(email)) return "invalid";

		return data.members.includes(email.toLowerCase()) ? "existing_member" : "pending";
	}

	function teamsFor(email: string): string[] {
		return teamIds[email] ?? (data.target.defaultTeamId ? [data.target.defaultTeamId] : []);
	}

	const composed = $derived<Invite[]>(
		parseAddresses($formData.addresses).map((email) => ({
			email,
			role: roles[email] ?? "member",
			teamIds: teamsFor(email),
			status: statusOf(email),
		}))
	);

	const rows = $derived<Invite[]>(
		(preview?.rows ?? sent ?? composed)
			.filter((row) => !removed.includes(row.email))
			.map((row) => ({
				...row,
				role: roles[row.email] ?? row.role,
				teamIds: teamIds[row.email] ?? row.teamIds,
			}))
	);

	const counts = $derived({
		total: rows.length,
		sendable: rows.filter((row) => row.status === "pending").length,
		linkOnly: rows.filter((row) => row.status === "link_only").length,
		invalid: rows.filter((row) => row.status === "invalid").length,
		attention: rows.filter(
			(row) => row.status === "invalid" || row.status === "existing_member"
		).length,
		sent: rows.filter((row) => row.status === "sent").length,
		failed: rows.filter((row) => row.status === "failed").length,
	});

	const working = $derived($submitting || settling);
	const sending = $derived(preview?.sending ?? (working && counts.total > 0));
	const busy = $derived(working || sending);
	const emailConfigured = $derived(preview?.emailConfigured ?? counts.linkOnly === 0);

	const allSent = $derived(
		counts.sent > 0 && counts.sendable === 0 && counts.linkOnly === 0 && counts.failed === 0
	);
	const composing = $derived(!sending && counts.sent === 0 && counts.failed === 0);

	const title = $derived(
		allSent ? `Invited ${counted(counts.sent, "person", "people")} to ${workspace}` : "Invite your team"
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
		if (counts.attention > 0) return counted(counts.total, "address", "addresses");
		return counted(counts.total, "person", "people");
	});

	const rowsRight = $derived.by(() => {
		if (sending) return { text: `${counts.sent} of ${counts.total} sent`, tone: "text-muted-foreground" };
		if (counts.failed > 0) return { text: `${counts.failed} failed`, tone: "text-destructive" };
		if (allSent) return { text: `${counts.sent} sent`, tone: "text-muted-foreground" };
		if (counts.attention > 0)
			return {
				text: `${counts.attention} need${counts.attention === 1 ? "s" : ""} attention`,
				tone: "text-warning",
			};
		return null;
	});

	const bounced = $derived.by(() => {
		if (counts.failed === 1) return "That address bounced at the provider. Everything else went out.";
		if (counts.failed === 2) return "Both addresses bounced at the provider. Everything else went out.";

		return "Those addresses bounced at the provider. Everything else went out.";
	});

	const notice = $derived.by(() => {
		if (unavailable) {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Could not send those invitations",
				body: "We couldn't reach the server just now. Nothing was created — wait a moment and try again.",
				action: null,
			};
		}
		if (!emailConfigured) {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Email delivery isn't configured",
				body: "This instance can't send anything until SMTP is set. Until then, copy each single-use link and share it yourself — they expire in seven days.",
				action: `Copy ${counted(counts.linkOnly, "invite link", "invite links")}`,
			};
		}
		if (counts.failed > 0) {
			return {
				variant: "warning" as const,
				icon: TriangleAlert,
				title: `${counted(counts.failed, "invitation", "invitations")} didn't send`,
				body: bounced,
				action: null,
			};
		}
		return null;
	});

	const cta = $derived.by(() => {
		if (sending) return "Sending";
		if (counts.failed > 0) return `Retry the ${counts.failed} that failed`;
		if (allSent) return "Continue";
		if (!emailConfigured && counts.linkOnly > 0)
			return `Copy ${counted(counts.linkOnly, "invite link", "invite links")}`;
		if (counts.sendable > 0) return `Send ${counted(counts.sendable, "invitation", "invitations")}`;
		return "Send invitations";
	});

	const secondary = $derived(sending ? null : allSent ? "Invite more" : "Skip for now");

	const footer = $derived(
		counts.invalid > 0
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

	const pendable = (status: InviteStatus) => status === "pending" || status === "link_only";
	const editable = (row: Invite) => !row.invitationId && pendable(row.status);
	const removable = (row: Invite) => pendable(row.status) || row.status === "invalid";
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-140">
		<div class="flex flex-col gap-4.5 p-6.5">
			<div class="flex flex-col gap-1.5">
				{#if !allSent}
					<Eyebrow>Step 4 of 4</Eyebrow>
				{/if}
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
						<Alert.Action placement="below">
							<Button variant="secondary" size="sm" disabled={busy} onclick={copyLinks}>
								<Copy aria-hidden="true" />
								{copied ? "Copied" : notice.action}
							</Button>
							<Button variant="ghost" size="sm" href="/settings/instance/mail">
								Configure SMTP
								<ExternalLink aria-hidden="true" />
							</Button>
						</Alert.Action>
					{/if}
				</Alert.Root>
			{/if}

			{#if composing}
				<form id={formId} method="POST" use:enhance>
					<Form.Field {form} name="addresses">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Email addresses</Form.Label>
								<Textarea
									{...props}
									rows={3}
									placeholder={"jun@northwind.co, milo@northwind.co\nada@northwind.co"}
									disabled={busy}
									bind:value={$formData.addresses}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
						<Form.Description>
							Paste a list — commas, spaces or line breaks all work.
						</Form.Description>
					</Form.Field>
				</form>
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
								class="flex min-h-8.5 flex-wrap items-center gap-2 border-b border-line-subtle py-1 pr-2 pl-2.5 last:border-b-0 sm:flex-nowrap"
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
									<span class="shrink-0 text-sm whitespace-nowrap {rowNoteTone[row.status]}">
										{rowNote[row.status]}
									</span>
								{/if}
								{#if editable(row) || removable(row)}
									<span class="ml-auto flex shrink-0 items-center gap-2 max-sm:ml-0 max-sm:w-full">
										{#if editable(row)}
											<span class="w-[106px] max-sm:min-w-0 max-sm:flex-1">
												<Select.Root
													type="single"
													value={row.role}
													disabled={busy}
													onValueChange={(value) => (roles[row.email] = value as MembershipRole)}
												>
													<Select.Trigger size="sm" aria-label="Role for {row.email}">
														{roleLabels[row.role]}
													</Select.Trigger>
													<Select.Content>
														{#each membershipRoles as role (role)}
															<Select.Item value={role} label={roleLabels[role]}>
																{roleLabels[role]}
															</Select.Item>
														{/each}
													</Select.Content>
												</Select.Root>
											</span>
											{#if data.teams.length > 0}
												<span class="w-[124px] max-sm:min-w-0 max-sm:flex-1">
													<Select.Root
														type="multiple"
														value={row.teamIds}
														disabled={busy}
														onValueChange={(value) => (teamIds[row.email] = value)}
													>
														<Select.Trigger size="sm" aria-label="Teams for {row.email}">
															{teamSummary(row.teamIds, data.teams)}
														</Select.Trigger>
														<Select.Content>
															{#each data.teams as team (team.id)}
																<Select.Item value={team.id} label={team.name}>
																	<TeamKey key={team.key} />
																	{team.name}
																</Select.Item>
															{/each}
														</Select.Content>
													</Select.Root>
												</span>
											{/if}
										{/if}
										{#if removable(row)}
											<Button
												variant="ghost"
												size="icon-xs"
												aria-label={row.invitationId
													? `Revoke the invitation for ${row.email}`
													: `Remove ${row.email}`}
												disabled={busy}
												onclick={() => remove(row)}
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
					<Button href={`/${data.target.slug}`}>{cta}</Button>
				{:else if counts.failed > 0}
					<Button disabled={busy} onclick={retryFailed}>{cta}</Button>
				{:else if !emailConfigured && counts.linkOnly > 0}
					<Button disabled={busy} onclick={copyLinks}>{copied ? "Copied" : cta}</Button>
				{:else}
					<Button type="submit" form={formId} disabled={busy || counts.sendable === 0}>
						{cta}
					</Button>
				{/if}
				{#if secondary}
					{#if allSent}
						<Button variant="ghost" onclick={inviteMore}>{secondary}</Button>
					{:else}
						<Button variant="ghost" href={`/${data.target.slug}`}>{secondary}</Button>
					{/if}
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
