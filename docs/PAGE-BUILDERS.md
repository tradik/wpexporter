# Page builder conversion rules

Complete `flat_html_rules` sets for the builders whose markup a generic converter cannot read on its own: Bricks, Elementor, Divi, Oxygen and GenerateBlocks. Copy the block for the builder the site uses into your config file — or combine several, which is what a site that changed builders once already needs.

Below are complete configuration examples for popular WordPress page builders.

## Bricks Builder

```yaml
# config-bricks.yaml
flat_html_rules:
  # Headings
  - class: "brxe-heading"
    tag: "div"
    markdown: "## {content}\n\n"
  - class: "brxe-heading"
    tag: "h1"
    markdown: "# {content}\n\n"
  - class: "brxe-heading"
    tag: "h2"
    markdown: "## {content}\n\n"
  - class: "brxe-heading"
    tag: "h3"
    markdown: "### {content}\n\n"

  # Text blocks
  - class: "brxe-text"
    markdown: "{content}\n\n"
  - class: "brxe-text-basic"
    markdown: "{content}\n\n"

  # Lists
  - class: "brxe-list"
    markdown: "{content}\n\n"

  # Buttons (extract as links)
  - class: "brxe-button"
    markdown: "[{content}]\n\n"

  # Code blocks
  - class: "brxe-code"
    markdown: "```\n{content}\n```\n\n"
```

## Elementor

```yaml
# config-elementor.yaml
flat_html_rules:
  # Headings
  - class: "elementor-heading-title"
    markdown: "## {content}\n\n"
  - class: "elementor-size-large"
    markdown: "# {content}\n\n"
  - class: "elementor-size-medium"
    markdown: "## {content}\n\n"
  - class: "elementor-size-small"
    markdown: "### {content}\n\n"

  # Text widgets
  - class: "elementor-text-editor"
    markdown: "{content}\n\n"
  - class: "elementor-widget-text-editor"
    markdown: "{content}\n\n"

  # Buttons
  - class: "elementor-button-text"
    markdown: "[{content}]\n\n"

  # Lists
  - class: "elementor-icon-list-text"
    markdown: "- {content}\n"

  # Testimonials
  - class: "elementor-testimonial-content"
    markdown: "> {content}\n\n"
  - class: "elementor-testimonial-name"
    markdown: "**{content}**\n\n"

  # Tabs and accordions
  - class: "elementor-tab-title"
    markdown: "### {content}\n\n"
  - class: "elementor-tab-content"
    markdown: "{content}\n\n"
  - class: "elementor-accordion-title"
    markdown: "### {content}\n\n"
  - class: "elementor-accordion-content"
    markdown: "{content}\n\n"
```

## Divi Builder

```yaml
# config-divi.yaml
flat_html_rules:
  # Module titles
  - class: "et_pb_module_header"
    markdown: "## {content}\n\n"

  # Text modules
  - class: "et_pb_text_inner"
    markdown: "{content}\n\n"

  # Blurb modules
  - class: "et_pb_blurb_content"
    markdown: "{content}\n\n"
  - class: "et_pb_blurb_title"
    markdown: "### {content}\n\n"

  # Buttons
  - class: "et_pb_button"
    markdown: "[{content}]\n\n"

  # Testimonials
  - class: "et_pb_testimonial_description"
    markdown: "> {content}\n\n"
  - class: "et_pb_testimonial_author"
    markdown: "**{content}**\n\n"

  # Tabs
  - class: "et_pb_tab_title"
    markdown: "### {content}\n\n"
  - class: "et_pb_tab_content"
    markdown: "{content}\n\n"

  # Toggle/Accordion
  - class: "et_pb_toggle_title"
    markdown: "### {content}\n\n"
  - class: "et_pb_toggle_content"
    markdown: "{content}\n\n"

  # Pricing tables
  - class: "et_pb_pricing_title"
    markdown: "### {content}\n\n"
  - class: "et_pb_pricing_content"
    markdown: "{content}\n\n"
```

## Oxygen Builder

```yaml
# config-oxygen.yaml
flat_html_rules:
  # Headings
  - class: "ct-headline"
    markdown: "## {content}\n\n"
  - class: "ct-headline"
    tag: "h1"
    markdown: "# {content}\n\n"
  - class: "ct-headline"
    tag: "h2"
    markdown: "## {content}\n\n"
  - class: "ct-headline"
    tag: "h3"
    markdown: "### {content}\n\n"

  # Text blocks
  - class: "ct-text-block"
    markdown: "{content}\n\n"

  # Rich text
  - class: "ct-rich-text"
    markdown: "{content}\n\n"

  # Buttons
  - class: "ct-button"
    markdown: "[{content}]\n\n"

  # Links
  - class: "ct-link-text"
    markdown: "{content}\n\n"
```

## GenerateBlocks

```yaml
# config-generateblocks.yaml
flat_html_rules:
  # Headlines
  - class: "gb-headline"
    markdown: "## {content}\n\n"
  - class: "gb-headline"
    tag: "h1"
    markdown: "# {content}\n\n"
  - class: "gb-headline"
    tag: "h2"
    markdown: "## {content}\n\n"
  - class: "gb-headline"
    tag: "h3"
    markdown: "### {content}\n\n"

  # Buttons
  - class: "gb-button"
    markdown: "[{content}]\n\n"
  - class: "gb-button-text"
    markdown: "{content}"
```

## Combining Multiple Page Builders

If your site uses multiple page builders or plugins, you can combine rules:

```yaml
# config-combined.yaml
flat_html_rules:
  # Bricks Builder
  - class: "brxe-heading"
    markdown: "## {content}\n\n"
  - class: "brxe-text"
    markdown: "{content}\n\n"

  # Elementor
  - class: "elementor-heading-title"
    markdown: "## {content}\n\n"
  - class: "elementor-text-editor"
    markdown: "{content}\n\n"

  # WPBakery/Visual Composer
  - class: "vc_custom_heading"
    markdown: "## {content}\n\n"
  - class: "wpb_text_column"
    markdown: "{content}\n\n"

  # Gutenberg blocks
  - class: "wp-block-heading"
    markdown: "## {content}\n\n"
  - class: "wp-block-paragraph"
    markdown: "{content}\n\n"
  - class: "wp-block-quote"
    markdown: "> {content}\n\n"
```
