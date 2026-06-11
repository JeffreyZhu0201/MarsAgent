FROM python:3.11-slim

WORKDIR /tmp

# Install common libraries (network available during build)
RUN pip install --no-cache-dir \
    numpy \
    pandas \
    requests \
    beautifulsoup4 \
    lxml \
    Pillow \
    scipy \
    sympy \
    networkx \
    matplotlib \
    scikit-learn \
    joblib \
    tqdm \
    pytz \
    python-dateutil \
    six \
    cycler \
    kiwisolver \
    packaging \
    pyparsing \
    fonttools \
    contourpy \
    importlib-resources \
    zipp \
    markdown-it-py \
    pygments \
    attrs \
    pyrsistent \
    jsonschema \
    ipython \
    ipython-genutils \
    decorator \
    jinja2 \
    markupsafe \
    typing-extensions \
    webencodings \
    bleach

# Clean pip cache
RUN rm -rf /root/.cache/pip/*

# Default command (will be overridden)
CMD ["python3"]
