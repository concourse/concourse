module TopBarTests exposing (all, userWithEmail, userWithId, userWithName, userWithUserName)

import Concourse
import Dict
import Expect exposing (..)
import Test exposing (..)
import TopBar exposing (userDisplayName)
import Navigation
import QueryString
import Routes


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
            , test "clicking a pinned resource navigates to the pinned resource page" <|
                \_ ->
                    TopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                        |> Tuple.first
                        |> TopBar.update (TopBar.GoToPinnedResource "resource")
                        |> Tuple.second
                        |> Expect.equal (Navigation.newUrl "/teams/team/pipelines/pipeline/resources/resource")
            , test "displays id if no userName, name or email present" <|
                \_ ->
                    Expect.equal
                        "some-id"
                        (TopBar.userDisplayName userWithId)
            ]
        ]
