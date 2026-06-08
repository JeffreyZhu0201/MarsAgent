from marsagent.collector import dedup


def test_compute_hashes_uses_installed_simhash_api():
    url_hash, content_hash = dedup.compute_hashes("hello world", "https://example.com")

    assert isinstance(url_hash, bytes)
    assert len(url_hash) == 32
    assert isinstance(content_hash, int)


def test_content_duplicate_uses_hamming_distance(monkeypatch):
    monkeypatch.setattr(dedup, "_known_simhashes", {0b1010: True})

    assert dedup.is_content_duplicate(0b1011)
    assert not dedup.is_content_duplicate(0xffffffffffffffff)
