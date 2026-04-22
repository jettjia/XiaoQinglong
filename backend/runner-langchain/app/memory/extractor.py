"""Memory extractor for extracting memories from conversations.

Based on Go runner's memory_extractor.go design.
Uses LLM to extract key information from conversations.
"""

import json
import logging
import threading
from dataclasses import dataclass
from typing import Optional

import httpx

logger = logging.getLogger(__name__)


@dataclass
class ExtractedMemory:
    """A memory extracted from conversation."""
    name: str
    description: str
    memory_type: str  # user, feedback, project, reference
    content: str
    importance: int = 2


class MemoryExtractor:
    """Extracts memories from conversations using LLM.

    Analyzes user input and assistant responses to extract:
    - user: User role, preferences, knowledge
    - feedback: User guidance/instructions
    - project: Project context
    - reference: External system pointers
    """

    def __init__(
        self,
        api_key: str,
        api_base: str = "https://api.openai.com/v1",
        model: str = "gpt-4o-mini",
    ):
        self.api_key = api_key
        self.api_base = api_base.rstrip("/")
        self.model = model

    def _build_extraction_prompt(self, user_input: str, assistant_output: str) -> str:
        """Build the prompt for memory extraction."""
        return f"""你是一个记忆提取专家。从以下对话中提取关键信息并以 JSON 格式返回。

对话:
用户: {user_input}
助手: {assistant_output}

记忆类型说明:
- user: 用户角色、偏好，知识 (如 "用户是数据科学家")
- feedback: 用户指导 (如 "用户说不要用 mock 测试")
- project: 项目上下文 (如 "项目截止日期是 3 月 15 日")
- reference: 外部系统指针 (如 "bug 在 Linear 的 INGEST 项目跟踪")

提取规则:
1. 只提取对话中明确提到的信息，不要推测
2. 每条记忆需要有: name(英文简短带下划线), description(描述), type(类型), content(完整内容)
3. 优先提取 feedback 类型（用户明确给过指导的）
4. 最多提取 5 条记忆
5. 如果没有值得记忆的信息，返回空数组 []

返回格式:
{{
  "memories": [
    {{"name": "user_role", "description": "用户是数据科学家，专注于日志系统", "type": "user", "content": "..."}},
    {{"name": "feedback_testing", "description": "不要使用 mock 数据库测试", "type": "feedback", "content": "..."}}
  ]
}}"""

    def _call_llm(self, prompt: str) -> str:
        """Call the LLM API."""
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
        }

        payload = {
            "model": self.model,
            "messages": [{"role": "user", "content": prompt}],
            "temperature": 0.3,
        }

        with httpx.Client(timeout=30.0) as client:
            response = client.post(
                f"{self.api_base}/chat/completions",
                headers=headers,
                json=payload,
            )

        if response.status_code != 200:
            raise RuntimeError(f"LLM returned status {response.status_code}: {response.text}")

        result = response.json()
        content = result["choices"][0]["message"]["content"]

        # Clean markdown code blocks
        content = content.strip()
        content = content.replace("```json\n", "", 1)
        content = content.replace("```\n", "", 1)
        content = content.strip("```").strip()

        logger.info("[Memory LLM] cleaned response: %s", content[:200])
        return content

    def extract_memories(
        self,
        user_input: str,
        assistant_output: str,
    ) -> list[ExtractedMemory]:
        """Extract memories from conversation.

        Args:
            user_input: The user's input message
            assistant_output: The assistant's response

        Returns:
            List of extracted memories
        """
        if not self.api_key or not self.model:
            return []

        try:
            prompt = self._build_extraction_prompt(user_input, assistant_output)
            result = self._call_llm(prompt)

            data = json.loads(result)
            memories_data = data.get("memories", [])

            memories = []
            for m in memories_data:
                memories.append(ExtractedMemory(
                    name=m.get("name", ""),
                    description=m.get("description", ""),
                    memory_type=m.get("type", "user"),
                    content=m.get("content", ""),
                    importance=2,
                ))

            return memories

        except Exception as e:
            logger.warning("Failed to extract memories: %s", e)
            return []

    def extract_memories_async(
        self,
        user_input: str,
        assistant_output: str,
        callback,
    ) -> None:
        """Extract memories asynchronously.

        Args:
            user_input: The user's input message
            assistant_output: The assistant's response
            callback: Function to call with extracted memories
        """
        def _worker():
            memories = self.extract_memories(user_input, assistant_output)
            if memories:
                callback(memories)

        thread = threading.Thread(target=_worker, daemon=True)
        thread.start()


def format_memories_for_log(memories: list[ExtractedMemory]) -> str:
    """Format memories for logging.

    Args:
        memories: List of extracted memories

    Returns:
        Formatted string for logging
    """
    if not memories:
        return ""

    lines = []
    for m in memories:
        lines.append(f"  - [{m.memory_type}] {m.name} ({m.description})")

    return "\n".join(lines)