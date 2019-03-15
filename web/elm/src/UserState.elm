module UserState exposing (UserState(..), isMember, map, user)

import Concourse
import Dict


type UserState
    = UserStateLoggedIn Concourse.User
    | UserStateLoggedOut
    | UserStateUnknown


map : (Concourse.User -> a -> b) -> UserState -> a -> Maybe b
map f userState =
    Maybe.map2 f (user userState) << Just


user : UserState -> Maybe Concourse.User
user userState =
    case userState of
        UserStateLoggedIn u ->
            Just u

        _ ->
            Nothing


isMember : { a | teamName : String, userState : UserState } -> Bool
isMember { teamName, userState } =
    case userState of
        UserStateLoggedIn user ->
            case Dict.get teamName user.teams of
                Just roles ->
                    List.member "member" roles
                        || List.member "owner" roles

                Nothing ->
                    False

        _ ->
            False
