module Dashboard.Group.Tag exposing (Tag(..), ordering, splitFirst, tag, view)

import Colors
import Concourse
import Dict
import Html exposing (Html)
import Html.Attributes exposing (style)
import List.Extra
import Ordering exposing (Ordering)


type Tag
    = Owner
    | Member
    | PipelineOperator
    | Viewer


ordering : Ordering (Maybe Tag)
ordering =
    Ordering.explicit
        [ Just Owner
        , Just Member
        , Just PipelineOperator
        , Just Viewer
        , Nothing
        ]


view : Bool -> Tag -> Html msg
view isHd t =
    Html.div
        ([ style "border" ("1px solid " ++ Colors.white)
         , style "display" "inline-block"
         , style "font-size" "0.7em"
         , style "padding" "0.5em"
         , style "line-height" "0.9em"
         , style "text-align" "center"
         , style "letter-spacing" "0.2em"
         ]
            ++ (if isHd then
                    [ style "margin-bottom" "1em" ]

                else
                    [ style "margin-bottom" "" ]
               )
        )
        [ Html.text <| toString t ]


toString : Tag -> String
toString t =
    case t of
        Owner ->
            "OWNER"

        Member ->
            "MEMBER"

        PipelineOperator ->
            "PIPELINE_OPERATOR"

        Viewer ->
            "VIEWER"


splitFirst : Char -> String -> String
splitFirst delim =
    String.toList
        >> List.Extra.takeWhile ((/=) delim)
        >> String.fromList


tag : Concourse.User -> String -> Maybe Tag
tag user teamName =
    Dict.get teamName user.teams
        |> Maybe.withDefault []
        |> List.map parseRole
        |> List.sortWith ordering
        |> List.head
        |> Maybe.withDefault Nothing


parseRole : String -> Maybe Tag
parseRole role =
    case role of
        "owner" ->
            Just Owner

        "member" ->
            Just Member

        "pipeline-operator" ->
            Just PipelineOperator

        "viewer" ->
            Just Viewer

        _ ->
            Nothing
