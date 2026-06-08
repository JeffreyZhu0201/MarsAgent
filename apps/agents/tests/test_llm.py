from marsagent.llm import response_text


class Block:
    def __init__(self, text):
        self.text = text


class Response:
    def __init__(self, content):
        self.content = content


def test_response_text_skips_provider_thinking_blocks():
    resp = Response([Block(None), Block('{"ok": true}')])

    assert response_text(resp) == '{"ok": true}'


def test_response_text_joins_multiple_visible_blocks():
    resp = Response([Block('first'), Block('second')])

    assert response_text(resp) == 'first\nsecond'
