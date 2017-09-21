module BuildDuration exposing (view, show)

import Date exposing (Date)
import Date.Format
import Duration exposing (Duration)
import Html exposing (Html)
import Html.Attributes exposing (class, title)
import Time exposing (Time)
import Concourse


view : Concourse.BuildDuration -> Time.Time -> Html a
view duration now =
    Html.table [ class "dictionary build-duration" ] <|
        case ( duration.startedAt, duration.finishedAt ) of
            ( Nothing, Nothing ) ->
                [ pendingLabel "pending" ]

            ( Just startedAt, Nothing ) ->
                [ labeledRelativeDate "started" now startedAt ]

            ( Nothing, Just finishedAt ) ->
                [ labeledRelativeDate "finished" now finishedAt ]

            ( Just startedAt, Just finishedAt ) ->
                let
                    durationElmIssue =
                        -- https://github.com/elm-lang/elm-compiler/issues/1223
                        Duration.between (Date.toTime startedAt) (Date.toTime finishedAt)
                in
                    [ labeledRelativeDate "started" now startedAt
                    , labeledRelativeDate "finished" now finishedAt
                    , labeledDuration "duration" durationElmIssue
                    ]


show : Concourse.BuildDuration -> Time.Time -> Html a
show duration now =
    Html.div [ class "build-duration with-gap" ] <|
        case duration.startedAt of
            Nothing ->
                []

            Just startedAt ->
                let
                    elapsed =
                        Duration.between (Date.toTime startedAt) now
                in
                    [ Html.text <| Duration.format elapsed ]


labeledRelativeDate : String -> Time -> Date -> Html a
labeledRelativeDate label now date =
    let
        ago =
            Duration.between (Date.toTime date) now
    in
        Html.tr []
            [ Html.td [ class "dict-key" ] [ Html.text label ]
            , Html.td
                [ title (Date.Format.format "%b %d %Y %I:%M:%S %p" date), class "dict-value" ]
                [ Html.span [] [ Html.text (Duration.format ago ++ " ago") ] ]
            ]


labeledDuration : String -> Duration -> Html a
labeledDuration label duration =
    Html.tr []
        [ Html.td [ class "dict-key" ] [ Html.text label ]
        , Html.td [ class "dict-value" ] [ Html.span [] [ Html.text (Duration.format duration) ] ]
        ]


pendingLabel : String -> Html a
pendingLabel label =
    Html.tr []
        [ Html.td [ class "dict-key" ] [ Html.text label ]
        , Html.td [ class "dict-value" ] []
        ]
