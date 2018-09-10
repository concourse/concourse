module Dashboard.Group.Tag exposing (..)

import Concourse
import Ordering exposing (Ordering)


type Tag
    = Exposed
    | Member


ordering : Ordering Tag
ordering =
    Ordering.explicit [ Member, Exposed ]


text : Tag -> String
text tag =
    case tag of
        Exposed ->
            "EXPOSED"

        Member ->
            "MEMBER"


tag : Concourse.User -> String -> Tag
tag user teamName =
    if List.member teamName user.teams then
        Member
    else
        Exposed
