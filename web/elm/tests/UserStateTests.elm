module UserStateTests exposing (all)

import Dict exposing (Dict)
import Expect
import Test exposing (Test, describe, test)
import UserState exposing (UserState(..), isAnonymous, isMember)


isMemberHelper : String -> Dict String (List String) -> Bool -> Bool
isMemberHelper teamName roles isAdmin =
    isMember
        { teamName = teamName
        , userState =
            UserStateLoggedIn
                { id = "test"
                , userName = "user"
                , name = "username"
                , email = "test_email"
                , isAdmin = isAdmin
                , teams = roles
                }
        }


all : Test
all =
    describe "user state"
        [ describe "isAnonymous" <|
            [ test "is true when the user is unknown" <|
                \_ ->
                    UserStateUnknown
                        |> isAnonymous
                        |> Expect.equal True
            , test "is false when the user is logged in" <|
                \_ ->
                    UserStateLoggedIn
                        { id = "test"
                        , userName = "user"
                        , name = "username"
                        , email = "test_email"
                        , isAdmin = True
                        , teams = Dict.fromList [ ( "team1", [ "role" ] ) ]
                        }
                        |> isAnonymous
                        |> Expect.false "logged-in user should not be anonymous"
            , test "is true when the user is logged out" <|
                \_ ->
                    UserStateLoggedOut
                        |> isAnonymous
                        |> Expect.true "logged-out user should be anonymous"
            ]
        , describe "isMember"
            [ test "is true when the super admin user is NOT the member on the given team" <|
                \_ ->
                    isMemberHelper "team1" (Dict.fromList [ ( "other-team", [ "owner" ] ) ]) True
                        |> Expect.equal True
            , test "is true when the member is the of role 'pipeline operator'" <|
                \_ ->
                    isMemberHelper "team1" (Dict.fromList [ ( "team1", [ "pipeline-operator" ] ) ]) False
                        |> Expect.equal True
            , test "is true when the member is the of role 'member'" <|
                \_ ->
                    isMemberHelper "team1" (Dict.fromList [ ( "team1", [ "member" ] ) ]) False
                        |> Expect.equal True
            , test "is true when the member is the of role 'owner'" <|
                \_ ->
                    isMemberHelper "team1" (Dict.fromList [ ( "team1", [ "owner" ] ) ]) False
                        |> Expect.equal True
            , test "is false when the member is NOT any of role 'owner' or 'member' or 'pipeline-operator' or admin" <|
                \_ ->
                    isMemberHelper "team1" (Dict.fromList [ ( "team1", [] ) ]) False
                        |> Expect.equal False
            ]
        ]
