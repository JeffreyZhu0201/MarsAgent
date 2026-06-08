"""所有 LLM prompt 模板。"""
from __future__ import annotations

PLANNER_SYSTEM = """你是一个课程规划专家。为给定主题设计一门计算机课程大纲。"""

PLANNER_USER = """主题：{topic}
受众：{audience}
难度：{depth}
参考 Wiki 知识库搜索结果：
{wiki_context}
请输出严格的 JSON 大纲（可直接 json.loads）：{{
  "outline": [
    {{
      "ch_id": "ch_01",
      "title": "章节标题",
      "objectives": ["学习目标1", "目标2"],
      "prereqs": ["先修章节/知识"],
      "est_min": 25,
      "bloom_level": "understand|apply|analyze",
      "key_concepts": ["核心概念1", "概念2"]
    }}
  ]
}}"""

AUTHOR_SYSTEM = """你是计算机课程讲师。根据大纲章节要求，写出专业讲义正文（Markdown）。"""

AUTHOR_USER = """章节：{ch_title}
学习目标：{objectives}
先修知识：{prereqs}
关键概念：{key_concepts}
参考 Wiki 内容（Top-{k} 相关片段）：
{context}

要求：
- 用中文撰写
- 包含清晰的子标题、代码示例、图表建议
- 引用 Wiki 中的来源（标注 [src: URL]"""

CODEENG_SYSTEM = """你是 Python/JS/Go 算法教师。为课程生成精选代码示例。"""

CODEENG_USER = """章节：{ch_title}
关键概念：{concepts}
讲义摘要：{summary}

生成 2-3 个代码示例（Python/JavaScript/Go 均可）。每个示例：{{"lang": "python", "title": "...", "code": "...", "expected_output": "..."}}"""

QUIZ_SYSTEM = """你是习题专家。按照 Bloom 分类法设计习题。"""

QUIZ_USER = """章节：{ch_title}
概念：{concepts}
讲义摘要：{summary}

生成 3 道题：1 MCQ + 1 填空 + 1 简答（答案附解析。输出 JSON 列表。"""

VALIDATOR_SYSTEM = """你是课程质量审计员。检查章节与大纲的对齐度、引用准确性、术语一致性。"""

VALIDATOR_USER = """章节：{ch_title}
大纲目标：{objectives}
讲义：{content_md}

检查：1) 覆盖了所有 objectives？ 2) 引用了 Wiki 来源？ 3) 无幻觉？
输出：{{"pass": true/false, "issues": ["issue1"], "suggestions": ["建议"]}}"""