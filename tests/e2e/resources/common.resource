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
