*** Settings ***
Resource    resources/common.resource
Suite Setup    Open Test Browser
Suite Teardown    Close Browser
Test Setup    Reserved Vault Fixture Must Be Absent
Test Teardown    Reserved Vault Fixture Must Be Absent

*** Test Cases ***
Canonical And Open Graph Metadata Match Every Site Page
    FOR    ${path}    ${canonical}    IN
    ...    /    https://engineering.paulheymann.de/
    ...    /a-small-deployment-pipeline-is-still-a-system/    https://engineering.paulheymann.de/a-small-deployment-pipeline-is-still-a-system/
    ...    /what-i-want-from-a-local-first-engineering-blog/    https://engineering.paulheymann.de/what-i-want-from-a-local-first-engineering-blog/
    ...    /impressum/    https://engineering.paulheymann.de/impressum/
    ...    /datenschutzerklaerung/    https://engineering.paulheymann.de/datenschutzerklaerung/
        Open Site Page    ${path}
        Get Attribute    css=link[rel="canonical"]    href    ==    ${canonical}
        Get Attribute    css=meta[property="og:title"]    content    !=    ${EMPTY}
        Get Attribute    css=meta[property="og:type"]    content    !=    ${EMPTY}
        Get Attribute    css=meta[property="og:url"]    content    ==    ${canonical}
        Get Attribute    css=meta[property="og:description"]    content    !=    ${EMPTY}
    END

Robots And Sitemap Are Available Through The Preview
    Open Site Page    /robots.txt
    Get Text    css=body    contains    Sitemap: https://engineering.paulheymann.de/sitemap.xml
    Open Site Page    /sitemap.xml
    Get Text    css=body    contains    https://engineering.paulheymann.de/a-small-deployment-pipeline-is-still-a-system/
    Get Text    css=body    contains    https://engineering.paulheymann.de/what-i-want-from-a-local-first-engineering-blog/

Visitors Receive No Cookies Or External Runtime Assets
    Open Site Page
    ${cookies}=    Get Cookies
    Should Be Empty    ${cookies}
    @{resource_urls}=    Evaluate JavaScript    ${None}    () => performance.getEntriesByType('resource').map((entry) => entry.name)
    FOR    ${url}    IN    @{resource_urls}
        Should Start With    ${url}    ${TARGET_URL}    Site-controlled pages must not load external runtime assets: ${url}
    END
