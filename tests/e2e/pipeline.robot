*** Settings ***
Resource    resources/common.resource
Suite Setup    Open Test Browser
Suite Teardown    Close Browser

*** Test Cases ***
Irrelevant Tags Do Not Generate A Preview
    ${content}=    Catenate    SEPARATOR=\n    \# Ignore this note    \#blog \#engineering    It has no preview tag.
    Write Pipeline Fixture    Ignore this note.md    ${content}
    Sleep    2 sec
    File Should Not Exist    ${PREVIEW_OUTPUT_DIRECTORY}/ignore-this-note/index.html
    Open Site Page
    ${body}=    Get Text    css=body
    Should Not Contain    ${body}    Ignore this note

Title Changes Preserve A Redirect To The Updated Post
    ${original}=    Catenate    SEPARATOR=\n    ---    date: 2026-09-05    ---    \# Pipeline Original    \#blog \#engineering \#preview    Original text.
    Write Pipeline Fixture    Pipeline Original.md    ${original}
    Wait For Preview Route    pipeline-original
    ${updated}=    Catenate    SEPARATOR=\n    ---    date: 2026-09-05    redirect:    ${SPACE}${SPACE}- /pipeline-original/    ---    \# Pipeline Updated    \#blog \#engineering \#preview    Updated text.
    Write Pipeline Fixture    Pipeline Updated.md    ${updated}
    Remove File    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}/Pipeline Original.md
    Wait For Preview Route    pipeline-updated
    Open Site Page    /pipeline-original/
    Get Url    ==    ${TARGET_URL}/pipeline-updated/
    Get Text    css=article h1    ==    Pipeline Updated

Wiki Links And Vault Images Render In The Preview
    ${target}=    Catenate    SEPARATOR=\n    ---    date: 2026-09-05    ---    \# Pipeline Target    \#blog \#engineering \#preview    Target text.
    ${source}=    Catenate    SEPARATOR=\n    ---    date: 2026-09-05    ---    \# Pipeline Source    \#blog \#engineering \#preview    See [[Pipeline Target|the target]].    ${EMPTY}    ![[pipeline-image.svg|Fixture image]]
    Write Pipeline Fixture    Pipeline Target.md    ${target}
    Write Pipeline Fixture    Pipeline Source.md    ${source}
    Wait For Preview Route    pipeline-source
    Open Site Page    /pipeline-source/
    Get Attribute    css=article a[href="/pipeline-target/"]    href    ==    /pipeline-target/
    Get Attribute    css=article img[alt="Fixture image"]    src    ==    /assets/posts/pipeline-image.svg

Validation Failure Leaves The Last Valid Preview Available
    ${valid}=    Catenate    SEPARATOR=\n    ---    date: 2026-09-05    ---    \# Pipeline Rollback    \#blog \#engineering \#preview    Valid snapshot.
    Write Pipeline Fixture    Pipeline Rollback.md    ${valid}
    Wait For Preview Route    pipeline-rollback
    ${invalid}=    Catenate    SEPARATOR=\n    \# Pipeline Rollback    \#blog \#engineering \#preview    Invalid snapshot.
    Write Pipeline Fixture    Pipeline Rollback.md    ${invalid}
    Sleep    2 sec
    Open Site Page    /pipeline-rollback/
    Get Text    css=article h1    ==    Pipeline Rollback
    ${journal}=    Run    journalctl -u engineering-blog-preview.service -n 80 --no-pager
    Should Contain    ${journal}    Pipeline Rollback.md: date property is required
