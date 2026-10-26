<script lang="ts" generics="T extends AgentAuthorityInput">
	import type { FormPath, SuperForm } from "sveltekit-superforms";
	import * as Form from "#lib/components/ui/form/index.js";
	import { Checkbox } from "#lib/components/ui/checkbox/index.js";
	import { cn } from "#lib/utils.js";
	import type { AgentAuthorityInput } from "./agent-authority-schema";
	import {
		agentPermissionGroups,
		agentPermissionLabels,
		agentPermissions,
		beyondGrant,
		type AgentPermission,
		type APIScope,
	} from "./agents";

	const withheldId = "agent-scopes-withheld";

	let {
		form,
		grantable,
		busy,
		inline = false,
	}: {
		form: SuperForm<T>;
		grantable: APIScope[] | null;
		busy: boolean;
		inline?: boolean;
	} = $props();

	// svelte-ignore state_referenced_locally
	const { form: formData } = form;

	const withheld = $derived(new Set(beyondGrant([...agentPermissions], grantable)));

	function locked(scope: AgentPermission): boolean {
		return busy || (withheld.has(scope) && !$formData.scopes.includes(scope));
	}

	function toggleScope(scope: AgentPermission, checked: boolean) {
		const next = new Set($formData.scopes);

		if (checked) next.add(scope);
		else next.delete(scope);

		formData.update((entered) => ({ ...entered, scopes: [...next] }));
	}
</script>

<Form.Fieldset {form} name={"scopes" as FormPath<T>}>
	<Form.Legend>Permissions</Form.Legend>
	<div class="grid gap-3 sm:grid-cols-2">
		{#each agentPermissionGroups as group (group.title)}
			<fieldset class="flex min-w-0 flex-col gap-1.5">
				<legend class="mb-2 text-xs font-medium tracking-snug text-ink-900">
					{group.title}
				</legend>
				{#each group.permissions as scope (scope)}
					<div class="flex items-start gap-2">
						{#if inline}
							<input
								id={`agent-scope-${scope}`}
								type="checkbox"
								name="scopes"
								value={scope}
								disabled={locked(scope)}
								checked={$formData.scopes.includes(scope)}
								onchange={(event) => toggleScope(scope, event.currentTarget.checked)}
								aria-describedby={withheld.has(scope) ? withheldId : undefined}
								class="mt-0.5 size-3.5 accent-ink-900"
							/>
						{:else}
							<Checkbox
								id={`agent-scope-${scope}`}
								disabled={locked(scope)}
								checked={$formData.scopes.includes(scope)}
								onCheckedChange={(checked) => toggleScope(scope, checked === true)}
								aria-describedby={withheld.has(scope) ? withheldId : undefined}
							/>
						{/if}
						<label
							for={`agent-scope-${scope}`}
							class={cn("text-xs leading-normal text-ink-600", withheld.has(scope) && "text-ink-400")}
						>
							{agentPermissionLabels[scope] ?? scope}
						</label>
					</div>
				{/each}
			</fieldset>
		{/each}
	</div>
	{#if withheld.size > 0}
		<p id={withheldId} class="text-xs leading-normal text-muted-foreground">
			An agent cannot do more than you can, so the greyed permissions need an administrator.
		</p>
	{/if}
	<Form.FieldErrors />
</Form.Fieldset>
