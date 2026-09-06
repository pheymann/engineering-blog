*** Settings ***
Library    Process

*** Test Cases ***
Publish Workflow Uses Isolated Repository And Remote
    ${command}=    Set Variable    go test -tags=integration ./cmd/vault-preview-service -run '^TestServicePublishWorkflowUsesOnlyTemporaryVaultAndRemote$' -count=1
    ${result}=    Run Process    sh    -c    ${command}    cwd=${EXECDIR}
    Should Be Equal As Integers    ${result.rc}    0
