<script lang="ts">
	import { Button } from "#lib/components/ui/button/index.js";
	import { providerLabel, type SignInWorkspace } from "./sign-in-step";
	import { ssoEntryPoint } from "./workspace-sign-in";

	let {
		workspaces,
		returnTo,
		preferred,
	}: { workspaces: SignInWorkspace[]; returnTo: string; preferred: boolean } = $props();
</script>

<ul class="flex flex-col gap-2">
	{#each workspaces as workspace (workspace.workspace)}
		<li>
			<Button
				href={ssoEntryPoint(workspace.workspace, returnTo)}
				variant={preferred ? "default" : "secondary"}
				class="w-full"
			>
				{providerLabel(workspace, workspaces.length)}
			</Button>
		</li>
	{/each}
</ul>
