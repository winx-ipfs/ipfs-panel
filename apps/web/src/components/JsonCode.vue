<script setup lang="ts">
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { json } from '@codemirror/lang-json'
import { bracketMatching, HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { tags } from '@lezer/highlight'
import { linter, type Diagnostic } from '@codemirror/lint'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView, drawSelection, keymap, lineNumbers } from '@codemirror/view'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { jsonErrorLine } from '@/config/model'

const model = defineModel<string>({ required: true })

const props = withDefaults(
  defineProps<{
    label: string
    invalidLabel: string
    readOnly?: boolean
  }>(),
  { readOnly: false },
)

const host = ref<HTMLElement | null>(null)
const labelConfig = new Compartment()
const editConfig = new Compartment()
let view: EditorView | null = null
let applying = false

function diagnostics(): Diagnostic[] {
  if (!view) return []
  const line = jsonErrorLine(view.state.doc.toString())
  if (line == null) return []
  const info = view.state.doc.line(Math.min(line, view.state.doc.lines))
  return [{ from: info.from, to: info.to, severity: 'error', message: props.invalidLabel }]
}

const highlight = HighlightStyle.define([
  { tag: [tags.string, tags.special(tags.string), tags.propertyName], color: 'var(--text)' },
  { tag: [tags.number, tags.bool, tags.null, tags.atom], color: 'var(--text)', fontWeight: '600' },
  { tag: [tags.punctuation, tags.bracket, tags.separator], color: 'var(--muted)' },
])

const theme = EditorView.theme({
  '&': {
    height: '100%',
    backgroundColor: 'var(--surface-sunken)',
    color: 'var(--text)',
    fontSize: '13px',
  },
  '.cm-scroller': {
    fontFamily: 'var(--font-mono)',
    lineHeight: '1.55',
  },
  '.cm-content': {
    padding: '8px 0',
    caretColor: 'var(--text)',
  },
  '.cm-gutters': {
    backgroundColor: 'var(--surface)',
    color: 'var(--muted)',
    borderRight: '1px solid var(--border)',
  },
  '.cm-activeLine': { backgroundColor: 'transparent' },
  '.cm-activeLineGutter': { color: 'var(--text)' },
  '&.cm-focused': { outline: 'none' },
  '.cm-selectionBackground, ::selection': {
    backgroundColor: 'var(--config-select)',
  },
  '.cm-lintRange-error': {
    backgroundImage: 'none',
    textDecoration: 'underline',
    textDecorationColor: 'var(--danger)',
  },
})

onMounted(() => {
  if (!host.value) return
  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: model.value,
      extensions: [
        lineNumbers(),
        drawSelection(),
        history(),
        bracketMatching(),
        syntaxHighlighting(highlight),
        json(),
        keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
        linter(diagnostics, { delay: 0 }),
        theme,
        editConfig.of(EditorView.editable.of(!props.readOnly)),
        labelConfig.of(EditorView.contentAttributes.of({ 'aria-label': props.label })),
        EditorView.updateListener.of((update) => {
          if (!update.docChanged || applying) return
          model.value = update.state.doc.toString()
        }),
      ],
    }),
  })
})

watch(
  () => model.value,
  (value) => {
    if (!view || value === view.state.doc.toString()) return
    applying = true
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    applying = false
  },
)

watch(
  () => props.readOnly,
  (readOnly) => {
    view?.dispatch({
      effects: editConfig.reconfigure(EditorView.editable.of(!readOnly)),
    })
  },
)

watch(
  () => props.label,
  (label) => {
    view?.dispatch({
      effects: labelConfig.reconfigure(EditorView.contentAttributes.of({ 'aria-label': label })),
    })
  },
)

onBeforeUnmount(() => {
  view?.destroy()
  view = null
})
</script>

<template>
  <div ref="host" class="config-code" />
</template>
