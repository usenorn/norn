<script lang="ts">
	import CheckRow from "./check-row.svelte";
	import type { EvidencePanel, IssueCheck } from "./checks";

	let {
		checks,
		slug,
		timezone,
		canManage,
		working,
		expanded,
		evidence,
		gapReferences,
		ontoggle,
		onfile,
		onwaive,
		ongap,
		onremove,
	}: {
		checks: IssueCheck[];
		slug: string;
		timezone: string;
		canManage: boolean;
		working: boolean;
		expanded: string[];
		evidence: Record<string, EvidencePanel>;
		gapReferences: Record<string, string>;
		ontoggle: (check: IssueCheck) => void;
		onfile: (check: IssueCheck) => void;
		onwaive: (check: IssueCheck) => void;
		ongap: (check: IssueCheck) => void;
		onremove: (check: IssueCheck) => void;
	} = $props();
</script>

<ul class="flex min-w-0 flex-col divide-y divide-line-subtle">
	{#each checks as check (check.id)}
		<CheckRow
			{check}
			{slug}
			{timezone}
			{canManage}
			{working}
			expanded={expanded.includes(check.id)}
			evidence={evidence[check.id] ?? { kind: "loading" }}
			gapReference={check.gapIssueId ? gapReferences[check.gapIssueId] : undefined}
			ontoggle={() => ontoggle(check)}
			onfile={() => onfile(check)}
			onwaive={() => onwaive(check)}
			ongap={() => ongap(check)}
			onremove={() => onremove(check)}
		/>
	{/each}
</ul>
