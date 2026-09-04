*** Settings ***
Resource    resources/common.resource
Suite Setup    Open Test Browser
Suite Teardown    Close Browser
Test Setup    Reserved Vault Fixture Must Be Absent
Test Teardown    Reserved Vault Fixture Must Be Absent

*** Test Cases ***
Both Mock Post Layouts Render Their Required Content
    FOR    ${path}    IN    /a-small-deployment-pipeline-is-still-a-system/    /what-i-want-from-a-local-first-engineering-blog/
        Open Site Page    ${path}
        Get Element Count    css=article.post h1    ==    1
        Get Element Count    css=article.post time    ==    1
        Get Element Count    css=article.post h2    >=    2
        Get Element Count    css=article.post p    >=    3
        Get Element Count    css=article.post a    >=    1
        Get Element Count    css=article.post li    >=    2
        Get Element Count    css=article.post img    >=    1
        Get Element Count    css=article.post blockquote    ==    1
        Get Element Count    css=article.post sub    >=    1
        Get Element Count    css=article.post pre code    ==    1
    END

Desktop And Mobile Layouts Avoid Horizontal Overflow
    Open Site Page
    Use Desktop Viewport
    ${desktop_margin}=    Evaluate JavaScript    css=.site-content    (element) => element.getBoundingClientRect().left
    Should Be True    ${desktop_margin} >= 32    Desktop side margin must be at least 2rem.
    Use Mobile Viewport
    ${mobile_margin}=    Evaluate JavaScript    css=.site-content    (element) => element.getBoundingClientRect().left
    Should Be Equal As Numbers    ${mobile_margin}    16    Mobile side margin must be reduced to 1rem.
    ${logo_center}=    Evaluate JavaScript    css=.site-logo    (element) => { const rect = element.getBoundingClientRect(); return rect.left + rect.width / 2; }
    ${viewport_center}=    Evaluate JavaScript    ${None}    () => window.innerWidth / 2
    Should Be True    abs(${logo_center} - ${viewport_center}) < 1    Mobile logo must be centered.
    ${scroll_width}=    Evaluate JavaScript    ${None}    () => document.documentElement.scrollWidth
    ${viewport_width}=    Evaluate JavaScript    ${None}    () => window.innerWidth
    Should Be True    ${scroll_width} <= ${viewport_width}    Mobile layout must not overflow horizontally.
