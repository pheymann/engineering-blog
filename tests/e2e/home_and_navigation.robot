*** Settings ***
Resource    resources/common.resource
Suite Setup    Open Test Browser
Suite Teardown    Close Browser
Test Setup    Reserved Vault Fixture Must Be Absent
Test Teardown    Reserved Vault Fixture Must Be Absent

*** Test Cases ***
Homepage Renders Newest First Cards With Five Line Excerpts
    Open Site Page
    Get Title    ==    Paul's Engineering Blog
    @{titles}=    Evaluate JavaScript    ${None}    () => [...document.querySelectorAll('.post-card h2')].map((element) => element.textContent.trim())
    Should Be Equal    ${titles}[0]    A small deployment pipeline is still a system
    Should Be Equal    ${titles}[1]    What I want from a local-first engineering blog
    ${card_count}=    Get Element Count    css=.post-card
    Should Be True    ${card_count} >= 2    The two ordering fixtures must be present alongside any real preview posts.
    FOR    ${index}    IN RANGE    0    ${card_count}
        ${break_count}=    Evaluate JavaScript    css=.post-card >> nth=${index}    (element) => element.querySelectorAll('.post-excerpt br').length
        Should Be Equal As Integers    ${break_count}    4    Each homepage excerpt must have five lines.
    END

Cards And Logo Navigate Within The Site
    Open Site Page
    Click    css=.post-card >> nth=0
    Get Url    ==    ${TARGET_URL}/a-small-deployment-pipeline-is-still-a-system/
    Get Text    css=article h1    ==    A small deployment pipeline is still a system
    Click    css=.site-logo
    Get Url    ==    ${TARGET_URL}/

Footer Links Navigate To Legal Pages
    Open Site Page
    Click    css=.footer-links a >> text=Impressum
    Get Url    ==    ${TARGET_URL}/impressum/
    Get Text    css=article h1    ==    Impressum
    Open Site Page
    Click    css=.footer-links a >> text=Datenschutzerklärung
    Get Url    ==    ${TARGET_URL}/datenschutzerklaerung/
    Get Text    css=article h1    ==    Datenschutzerklärung
