module UserState exposing (..)

import Concourse


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
