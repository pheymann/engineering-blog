*** Settings ***
Library    Browser
Library    OperatingSystem

*** Keywords ***
Open Test Browser
    New Browser    ${BROWSER}    headless=${HEADLESS}    executablePath=${BROWSER_EXECUTABLE}
    New Context    viewport={"width": ${DESKTOP_WIDTH}, "height": ${DESKTOP_HEIGHT}}
    New Page

Open Site Page
    [Arguments]    ${path}=/
    Go To    ${TARGET_URL}${path}
    Wait For Elements State    css=body    visible

Use Desktop Viewport
    Set Viewport Size    ${DESKTOP_WIDTH}    ${DESKTOP_HEIGHT}

Use Mobile Viewport
    Set Viewport Size    ${MOBILE_WIDTH}    ${MOBILE_HEIGHT}

Reserved Vault Fixture Must Be Absent
    File Should Not Exist    ${VAULT_TEST_FIXTURE}

Prepare Pipeline Fixture Suite
    Directory Should Not Exist    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}
    Create Directory    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}
    Copy File    ${EXECDIR}/src/site/assets/images/static-publishing-path.svg    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}/pipeline-image.svg
    ${newer}=    Catenate    SEPARATOR=\n    ---    date: 2099-09-04    ---    \# A small deployment pipeline is still a system    \#blog \#engineering \#preview    \#\# A small system    \#\# Another section    A first paragraph with [a link](/).    A second paragraph.    - one    - two    ![Pipeline image](pipeline-image.svg)    > A useful quote    ~1~    ```text    code    ```
    ${older}=    Catenate    SEPARATOR=\n    ---    date: 2099-09-03    ---    \# What I want from a local-first engineering blog    \#blog \#engineering \#preview    \#\# A local-first note    \#\# Another section    A first paragraph with [a link](/).    A second paragraph.    - one    - two    ![Pipeline image](pipeline-image.svg)    > A useful quote    ~2~    ```text    code    ```
    Create File    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}/A small deployment pipeline is still a system.md    ${newer}
    Create File    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}/What I want from a local-first engineering blog.md    ${older}
    Wait Until Keyword Succeeds    20 sec    500 ms    File Should Exist    ${PREVIEW_OUTPUT_DIRECTORY}/a-small-deployment-pipeline-is-still-a-system/index.html
    Wait Until Keyword Succeeds    20 sec    500 ms    File Should Exist    ${PREVIEW_OUTPUT_DIRECTORY}/what-i-want-from-a-local-first-engineering-blog/index.html

Cleanup Pipeline Fixture Suite
    Remove Directory    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}    recursive=True
    Wait Until Keyword Succeeds    40 sec    500 ms    File Should Not Exist    ${PREVIEW_OUTPUT_DIRECTORY}/a-small-deployment-pipeline-is-still-a-system/index.html
    Wait Until Keyword Succeeds    40 sec    500 ms    File Should Not Exist    ${PREVIEW_OUTPUT_DIRECTORY}/what-i-want-from-a-local-first-engineering-blog/index.html

Write Pipeline Fixture
    [Arguments]    ${name}    ${content}
    Create File    ${VAULT_PIPELINE_FIXTURE_DIRECTORY}/${name}    ${content}

Wait For Preview Route
    [Arguments]    ${slug}
    Wait Until Keyword Succeeds    20 sec    500 ms    File Should Exist    ${PREVIEW_OUTPUT_DIRECTORY}/${slug}/index.html

Wait For Preview Route To Be Removed
    [Arguments]    ${slug}
    Wait Until Keyword Succeeds    20 sec    500 ms    File Should Not Exist    ${PREVIEW_OUTPUT_DIRECTORY}/${slug}/index.html
