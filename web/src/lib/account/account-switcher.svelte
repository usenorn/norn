<script lang="ts">
	import { page } from "$app/state";
	import Check from "@lucide/svelte/icons/check";
	import ChevronsUpDown from "@lucide/svelte/icons/chevrons-up-down";
	import LogOut from "@lucide/svelte/icons/log-out";
	import Plus from "@lucide/svelte/icons/plus";
	import Settings from "@lucide/svelte/icons/settings";
	import Terminal from "@lucide/svelte/icons/terminal";
	import UserPlus from "@lucide/svelte/icons/user-plus";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import WorkspaceMark from "$lib/components/norn/workspace-mark.svelte";
	import AccountIdentity from "$lib/account/account-identity.svelte";
	import { initialsOf, withSlot, type SignedInAccount } from "$lib/account/accounts";
	import { workspacePath } from "$lib/workspace/navigation";

	type WorkspaceContext = { slug: string; name: string };

	let {
		accounts,
		actingAccountId,
		workspace,
		trigger = "workspace",
		context = "application",
		class: className,
	}: {
		accounts: SignedInAccount[];
		actingAccountId: string;
		workspace?: WorkspaceContext;
		trigger?: "workspace" | "avatar" | "person";
		context?: "application" | "workspace-settings" | "user-settings";
		class?: string;
	} = $props();

	const acting = $derived(accounts.find((signedIn) => signedIn.account.id === actingAccountId));
	const currentSlug = $derived(workspace?.slug ?? "");
	const returnTo = $derived(encodeURIComponent(page.url.pathname + page.url.search));
	const addAccountHref = $derived(`/sign-in?add=1&return=${returnTo}`);
	const workspaceDestination = $derived(
		context === "workspace-settings" ? "/settings" : "/my-tasks"
	);

	function signOutForm(accountId: string): string {
		return `sign-out-${accountId}`;
	}
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		class={[
			"flex items-center text-left motion-control focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
			trigger === "avatar"
				? "rounded-full"
				: "h-8.5 w-full gap-2 rounded-md px-1.5 hover:bg-accent",
			className,
		]}
		aria-label={trigger === "person" ? "Your account" : "Switch account or workspace"}
	>
		{#if trigger === "workspace" && workspace}
			<WorkspaceMark name={workspace.name} />
			<span class="min-w-0 flex-1 truncate text-md font-medium tracking-snug text-ink-900">
				{workspace.name}
			</span>
			<ChevronsUpDown class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
		{:else}
			<Avatar.Root size="sm">
				{#if acting?.account.avatarUrl}
					<Avatar.Image src={acting.account.avatarUrl} alt="" />
				{/if}
				<Avatar.Fallback>{initialsOf(acting?.account.displayName ?? "")}</Avatar.Fallback>
			</Avatar.Root>

			{#if trigger === "person"}
				<span class="min-w-0 flex-1 truncate text-sm text-ink-600">
					{acting?.account.displayName}
				</span>
				<ChevronsUpDown class="size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />
			{/if}
		{/if}
	</DropdownMenu.Trigger>

	<DropdownMenu.Content
		align="start"
		side={context === "user-settings" ? "bottom" : trigger === "person" ? "top" : "bottom"}
		sideOffset={4}
		width="menu"
	>
		{#if context === "user-settings"}
			{#each accounts as signedIn (signedIn.account.id)}
				<DropdownMenu.Group>
					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a href={withSlot("/settings/tokens", signedIn.defaultSlot)} {...props}>
								<AccountIdentity account={signedIn.account} />
								{#if signedIn.account.id === actingAccountId}
									<Check class="text-ink-600" aria-hidden="true" />
								{/if}
							</a>
						{/snippet}
					</DropdownMenu.Item>
				</DropdownMenu.Group>
			{/each}
			<DropdownMenu.Separator />
		{:else}
			{#each accounts as signedIn (signedIn.account.id)}
				<DropdownMenu.Group>
					<DropdownMenu.GroupHeading class="flex items-center justify-between gap-2 px-2 py-1.5">
						<AccountIdentity account={signedIn.account} />
						{#if signedIn.account.id === actingAccountId}
							<Check class="size-icon-row shrink-0 text-ink-600" aria-hidden="true" />
						{/if}
					</DropdownMenu.GroupHeading>

					{#each signedIn.workspaces as reach (reach.workspace.id)}
						<DropdownMenu.Item>
							{#snippet child({ props })}
								<a
									href={withSlot(
										workspacePath(reach.workspace.slug, workspaceDestination),
										reach.slot
									)}
									{...props}
								>
									<WorkspaceMark
										name={reach.workspace.name}
										class="size-4.5 rounded-xs text-2xs"
									/>
									<span class="min-w-0 flex-1 truncate">{reach.workspace.name}</span>
									{#if reach.workspace.slug === currentSlug && signedIn.account.id === actingAccountId}
										<Check class="text-ink-600" aria-hidden="true" />
									{/if}
								</a>
							{/snippet}
						</DropdownMenu.Item>
					{/each}

					{#if signedIn.workspaces.length === 0}
						<DropdownMenu.Item disabled>No workspaces yet</DropdownMenu.Item>
					{/if}

					<DropdownMenu.Item>
						{#snippet child({ props })}
							<a href={withSlot("/create-workspace", signedIn.defaultSlot)} {...props}>
								<Plus aria-hidden="true" />
								New workspace
							</a>
						{/snippet}
					</DropdownMenu.Item>

					<DropdownMenu.Sub>
						<DropdownMenu.SubTrigger>User settings</DropdownMenu.SubTrigger>
						<DropdownMenu.SubContent>
							<DropdownMenu.Item>
								{#snippet child({ props })}
									<a href={withSlot("/settings/tokens", signedIn.defaultSlot)} {...props}>
										<Terminal aria-hidden="true" />
										Your API tokens
									</a>
								{/snippet}
							</DropdownMenu.Item>
							<DropdownMenu.Item>
								{#snippet child({ props })}
									<a href={withSlot("/create-workspace", signedIn.defaultSlot)} {...props}>
										<Plus aria-hidden="true" />
										Create a workspace
									</a>
								{/snippet}
							</DropdownMenu.Item>
							<DropdownMenu.Separator />
							<DropdownMenu.Item variant="destructive">
								{#snippet child({ props })}
									<button type="submit" form={signOutForm(signedIn.account.id)} {...props}>
										<LogOut aria-hidden="true" />
										Sign out {signedIn.account.email}
									</button>
								{/snippet}
							</DropdownMenu.Item>
						</DropdownMenu.SubContent>
					</DropdownMenu.Sub>
				</DropdownMenu.Group>
				<DropdownMenu.Separator />
			{/each}
		{/if}

		{#if workspace && context === "application"}
			<DropdownMenu.Group>
				<DropdownMenu.Item>
					{#snippet child({ props })}
						<a href={workspacePath(currentSlug, "/settings")} {...props}>
							<Settings aria-hidden="true" />
							Workspace settings
						</a>
					{/snippet}
				</DropdownMenu.Item>
			</DropdownMenu.Group>
			<DropdownMenu.Separator />
		{/if}

		<DropdownMenu.Item>
			{#snippet child({ props })}
				<a href={addAccountHref} {...props}>
					<UserPlus aria-hidden="true" />
					Add another account
				</a>
			{/snippet}
		</DropdownMenu.Item>

		<DropdownMenu.Separator />

		{#if acting}
			<DropdownMenu.Item variant="destructive">
				{#snippet child({ props })}
					<button type="submit" form={signOutForm(acting.account.id)} {...props}>
						<LogOut aria-hidden="true" />
						Sign out
					</button>
				{/snippet}
			</DropdownMenu.Item>
		{/if}

		{#if accounts.length > 1}
			<DropdownMenu.Item variant="destructive">
				{#snippet child({ props })}
					<button type="submit" form="sign-out-all" {...props}>
						<LogOut aria-hidden="true" />
						Sign out of all accounts
					</button>
				{/snippet}
			</DropdownMenu.Item>
		{/if}
	</DropdownMenu.Content>
</DropdownMenu.Root>

{#each accounts as signedIn (signedIn.account.id)}
	<form
		id={signOutForm(signedIn.account.id)}
		method="POST"
		action={withSlot("/sign-out", signedIn.defaultSlot)}
		class="hidden"
	></form>
{/each}
<form id="sign-out-all" method="POST" action="/sign-out?scope=all" class="hidden"></form>
