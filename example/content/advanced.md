---
title: Advanced examples
---

Named citation: {{< cite keys="paper" prefix="advanced-" >}}.

Short form: {{< cite keys="ruby" prefix="advanced-" suppress_author=true locator="42" >}}.

Two links in one citation: {{< cite keys="ruby paper" prefix="advanced-" separate_links=true >}}.

Bibliography ({{< bibliography_count type="book" >}} book):

{{< bibliography prefix="advanced-" >}}

Single reference: {{< reference key="paper" >}}

Details: {{< cite_details key="paper" text="Read paper details" >}}

> See [the book details]({{< details_link "ruby" >}}).

{{< quote key="ruby" prefix="advanced-" >}}A quoted passage with a source.{{< /quote >}}

Historical publications ({{< bibliography_count query="@*[year<1900]" >}}):

{{< bibliography query="@*[year<1900]" prefix="old-" >}}

Query checks: {{< bibliography_count query="@book[year>=2000 && author ^= Flanagan]" >}} recent book; {{< bibliography_count query="!@book" >}} other item.

Two selectors: {{< bibliography_count query="@book, @inproceedings" >}} entries.

Uncited in text but included in the cited list: {{< nocite "paper" >}}
