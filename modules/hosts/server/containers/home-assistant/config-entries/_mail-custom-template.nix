# Shared `custom_event_data_template` for both Gmail IMAP entries.
#
# Two jobs, one template, because the imap coordinator renders exactly one:
#   unsub   — the RFC 8058 one-click URL, or "" when the sender doesn't offer one
#   preview — first ~200 chars of the body, tags stripped and whitespace collapsed
#
# `text` is injected into the template context unconditionally and is NOT capped by
# max_message_size there (only the event copy is), so the preview can be cut here
# while the body itself never enters the event bus. `headers` is different: it is
# only in scope when "headers" is listed in event_message_data — hence both entries
# set it, and hence the whole header block rides along in the event.
#
# The unsub gate is deliberately narrow: an https URL AND List-Unsubscribe-Post
# advertising One-Click. mailto: and bare GET links are dropped, because fetching
# those confirms to a spammer that the address is live.
''
  {%- set ns = namespace(lu="", post="") -%}
  {%- for k, v in (headers | default({})).items() -%}
    {%- if k | lower == 'list-unsubscribe' -%}{%- set ns.lu = v | join(' ') -%}
    {%- elif k | lower == 'list-unsubscribe-post' -%}{%- set ns.post = v | join(' ') -%}
    {%- endif -%}
  {%- endfor -%}
  {%- set urls = ns.lu | regex_findall('<(https://[^>]+)>') -%}
  {%- set ok = 'One-Click' in ns.post and (urls | count) > 0 -%}
  {%- set body = (text | default("")) | regex_replace('<[^>]*>', ' ') | regex_replace('\s+', ' ') -%}
  {{ {'unsub': (urls[0] if ok else ""),
      'preview': body | trim | truncate(200, true, '…')} | to_json }}
''
