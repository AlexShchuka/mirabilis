---
category: sandbox-ops
memory_type: procedural
summary: How to operate this container: tools, boundaries, build and run commands, gotchas.
max_lines: 80
---

# Sandbox Ops

- PDF-gen в sandbox [recipe, verified 2026-06-21, see [[research-log]] Bogdan-PDF]: NO converters present (pandoc/wkhtmltopdf/weasyprint/libreoffice/gs/poppler/pdfinfo ALL absent) ∧ no Cyrillic system fonts (fc-list пусто). WORKING path:
  1. `pip install --break-system-packages --target=/workspace/.pdfbuild/libs fpdf2` (venv не поднялся: PEP668). target MUST be on /workspace, NOT /tmp — /tmp = noexec ⇒ .so «failed to map segment from shared object» (fontTools/Pillow native ext).
  2. Cyrillic font: тянуть DejaVuSans.ttf + DejaVuSans-Bold.ttf c raw.githubusercontent.com/matplotlib/matplotlib/main/lib/matplotlib/mpl-data/fonts/ttf/ (matplotlib bundles full-Cyrillic DejaVu ; шрифт = data-only ⇒ /tmp/fonts ok).
  3. build: fpdf2 `add_font("DejaVu","",ttf)` + `("DejaVu","B",bold)`, `.table()` API для таблиц. GOTCHA: `FPDF.footer()` cell оставляет x у правого поля ⇒ после auto-page-break следующий multi_cell кидает «Not enough horizontal space to render a single character» — fix = `new_x="LMARGIN"` в footer + `set_x(l_margin)` в начале хелперов.
  4. render-verify (нет poppler/gs): `pip install --break-system-packages --target=... pymupdf` (self-contained MuPDF, no system deps), `fitz.open(pdf); doc[i].get_pixmap(dpi=110).save(png)` → Read png глазами (проверить кириллицу ≠ квадраты).
  - net: pip→pypi и raw.githubusercontent доступны (200).
- memory-prune [owner-mandate 2026-06-22 ; anchor-pedagogy]: research-log append-only пухнет (68/80 cap) → РЕЗАТЬ/compact периодически (= его же файл-политика «compacted periodically»). mnemonic(owner) = Джон Уик (Киану Ривз) ⊕ Стэтхем = экранные мясники ⇒ «резня» лога, чтоб ¬забыл. EXEC-rule: режу по отмашке owner (ЧТО+сколько), ¬auto (НЕ ТОРОПИТЬСЯ). see [[research-log]].
