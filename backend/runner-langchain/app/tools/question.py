"""AskUserQuestion tool for user interaction.

Based on Go runner's question.go design.
Asks user multiple choice questions to gather information.
"""

import json
from typing import Optional

from app.tools.registry import get_registry


def _ask_user_question(
    question: str,
    options: list[dict],
    header: str = "",
    multi_select: bool = False,
) -> str:
    """Ask the user a multiple choice question.

    Args:
        question: The question to ask
        options: List of options (each with label and optional description)
        header: Short header for the question
        multi_select: Allow multiple selections

    Returns:
        JSON with question details for the system to handle
    """
    try:
        if len(options) < 2:
            return json.dumps({"error": "At least 2 options required"})
        if len(options) > 4:
            return json.dumps({"error": "Maximum 4 options allowed"})

        # Build options description
        options_desc = []
        for i, opt in enumerate(options):
            label = opt.get("label", "")
            desc = opt.get("description", "")
            opt_str = f"{i + 1}. {label}"
            if desc:
                opt_str += f" - {desc}"
            options_desc.append(opt_str)

        # Return structured question for the system to present
        return json.dumps({
            "question": question,
            "options": [
                {"label": opt.get("label", ""), "description": opt.get("description", "")}
                for opt in options
            ],
            "header": header,
            "multi_select": multi_select,
            "options_formatted": options_desc,
        })

    except Exception as e:
        return json.dumps({"error": str(e)})


def register_tools() -> None:
    """Register ask_user_question tool."""
    registry = get_registry()

    registry.register(
        name="ask_user_question",
        description="Ask the user multiple choice questions to gather information, clarify ambiguity, or understand preferences.",
        schema={
            "type": "object",
            "properties": {
                "question": {
                    "type": "string",
                    "description": "The question to ask the user",
                },
                "options": {
                    "type": "array",
                    "description": "Answer options (each with label and optional description)",
                    "items": {
                        "type": "object",
                        "properties": {
                            "label": {"type": "string"},
                            "description": {"type": "string"},
                        },
                        "required": ["label"],
                    },
                },
                "header": {
                    "type": "string",
                    "description": "Short header/chip for the question (max 12 chars)",
                },
                "multi_select": {
                    "type": "boolean",
                    "description": "Allow multiple selections (default false)",
                },
            },
            "required": ["question", "options"],
        },
        handler=lambda **kwargs: _ask_user_question(
            question=kwargs.get("question", ""),
            options=kwargs.get("options", []),
            header=kwargs.get("header", ""),
            multi_select=kwargs.get("multi_select", False),
        ),
    )