<script lang="ts">
	import { diffCodeTones, diffGutterTones, diffMarks } from "#lib/executions/diff.js";
	import type { RevisionDiff } from "./diff";

	let { diff, first = false }: { diff: RevisionDiff; first?: boolean } = $props();
</script>

{#if diff.additions + diff.deletions === 0}
	<p class="px-3 py-3 text-xs text-muted-foreground">
		{first ? "Nothing was written in this version." : "The text did not change in this version."}
	</p>
{:else}
	<div class="flex flex-col">
		<p class="flex items-center gap-2 border-b border-line-subtle px-3 py-1.5 font-mono text-2xs">
			<span class="text-success">+{diff.additions}</span>
			<span class="text-destructive">−{diff.deletions}</span>
		</p>
		<table class="w-full table-fixed border-collapse font-mono text-2xs leading-relaxed">
			<caption class="sr-only">Lines added and removed in this version</caption>
			<colgroup>
				<col class="w-8 sm:w-10" />
				<col class="w-8 sm:w-10" />
				<col />
			</colgroup>
			<tbody>
				{#each diff.lines as line, index (index)}
					<tr>
						<td class="px-1.5 text-right select-none {diffGutterTones[line.kind]}">{line.oldLine ?? ""}</td>
						<td class="px-1.5 text-right select-none {diffGutterTones[line.kind]}">{line.newLine ?? ""}</td>
						<td class="px-2 break-words whitespace-pre-wrap {diffCodeTones[line.kind]}"
							><span class="select-none text-muted-foreground" aria-hidden="true">{diffMarks[line.kind]}</span
							><span class="sr-only">{line.kind === "add" ? "Added: " : line.kind === "remove" ? "Removed: " : ""}</span
							>{line.text}</td
						>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
