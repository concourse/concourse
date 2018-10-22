module TopBarTests exposing (all, userWithEmail, userWithId, userWithName, userWithUserName)

import Concourse
import Dict
import Expect exposing (..)
import Test exposing (..)
import TopBar exposing (userDisplayName)


userWithId : Concourse.User
userWithId =
    { id = "some-id", email = "", name = "", userName = "", teams = Dict.empty }


userWithEmail : Concourse.User
userWithEmail =
    { id = "some-id", email = "some-email", name = "", userName = "", teams = Dict.empty }


userWithName : Concourse.User
userWithName =
    { id = "some-id", email = "some-email", name = "some-name", userName = "", teams = Dict.empty }


userWithUserName : Concourse.User
userWithUserName =
    { id = "some-id", email = "some-email", name = "some-name", userName = "some-user-name", teams = Dict.empty }


all : Test
all =
    describe "TopBar"
        [ describe "userDisplayName"
            [ test "displays user name if present" <|
                \_ ->
                    Expect.equal
                        "some-user-name"
                        (TopBar.userDisplayName userWithUserName)
            , test "displays name if no userName present" <|
                \_ ->
                    Expect.equal
                        "some-name"
                        (TopBar.userDisplayName userWithName)
            , test "displays email if no userName or name present" <|
                \_ ->
                    Expect.equal
                        "some-email"
                        (TopBar.userDisplayName userWithEmail)
            , test "displays id if no userName, name or email present" <|
                \_ ->
                    Expect.equal
                        "some-id"
                        (TopBar.userDisplayName userWithId)
            ]
        ]
