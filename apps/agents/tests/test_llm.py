from marsagent.llm import extract_thinking, response_text


class Block:
    def __init__(self, text, type=None, thinking=None):
        self.text = text
        self.type = type
        self.thinking = thinking

    def __repr__(self):
        return f"Block(text={self.text!r}, type={self.type!r}, thinking={self.thinking!r})"


class Response:
    def __init__(self, content):
        self.content = content


def test_response_text_skips_provider_thinking_blocks():
    resp = Response([Block(None), Block('{"ok": true}')])

    assert response_text(resp) == '{"ok": true}'


def test_response_text_joins_multiple_visible_blocks():
    resp = Response([Block('first'), Block('second')])

    assert response_text(resp) == 'first\nsecond'


def test_extract_thinking_returns_thinking_content():
    # ThinkingBlock has type='thinking' and .thinking attribute (not .text)
    thinking_block = Block(None, type='thinking', thinking='Let me plan the course structure... consider the audience level...')
    text_block = Block('{"outline": [{"ch_id": "ch1", "title": "Intro"}]}')
    resp = Response([thinking_block, text_block])

    assert extract_thinking(resp) == 'Let me plan the course structure... consider the audience level...'


def test_extract_thinking_returns_empty_when_no_thinking():
    text_block = Block('{"ok": true}')
    resp = Response([text_block])

    assert extract_thinking(resp) == ''


def test_extract_thinking_joins_multiple_thinking_blocks():
    t1 = Block(None, type='thinking', thinking='First thought...')
    t2 = Block(None, type='thinking', thinking='Continuing...')
    resp = Response([t1, t2])

    assert extract_thinking(resp) == 'First thought...\nContinuing...'
