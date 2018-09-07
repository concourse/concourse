module Dashboard.Group.Tag exposing (..)

import Concourse
import Ordering exposing (Ordering)


type Tag
    = Public
    | Member


ordering : Ordering Tag
ordering =
    Ordering.explicit [ Member, Public ]


text : Tag -> String
text tag =
    case tag of
        Public ->
            "PUBLIC"

        Member ->
            "MEMBER"


tag : Concourse.User -> String -> Tag
tag user teamName =
    if List.member teamName user.teams then
        Member
    else
        Public
