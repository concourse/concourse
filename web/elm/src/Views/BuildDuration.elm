module Views.BuildDuration exposing (show, view)

import Concourse
import DateFormat
import Duration exposing (Duration)
import Html exposing (Html)
import Html.Attributes exposing (class, title)
import Time


view : Concourse.BuildDuration -> Time.Posix -> Html a
view duration now =
    Html.table [ class "dictionary build-duration" ] <|
        case ( duration.startedAt, duration.finishedAt ) of
            ( Nothing, Nothing ) ->
                [ pendingLabel "pending" ]

            ( Just startedAt, Nothing ) ->
                [ labeledDate "started" now startedAt ]

            ( Nothing, Just finishedAt ) ->
                [ labeledDate "finished" now finishedAt ]

            ( Just startedAt, Just finishedAt ) ->
                let
                    durationElmIssue =
                        -- https://github.com/elm-lang/elm-compiler/issues/1223
                        Duration.between startedAt finishedAt
                in
                [ labeledDate "started" now startedAt
                , labeledDate "finished" now finishedAt
                , labeledDuration "duration" durationElmIssue
                ]


show : Time.Posix -> Concourse.BuildDuration -> String
show now =
    .startedAt
        >> Maybe.map ((\a -> Duration.between a now) >> Duration.format)
        >> Maybe.withDefault ""


labeledDate : String -> Time.Posix -> Time.Posix -> Html a
labeledDate label now date =
    let
        ago =
            Duration.between date now

        verboseDate =
            DateFormat.format
                [ DateFormat.monthNameAbbreviated
                , DateFormat.text " "
                , DateFormat.dayOfMonthNumber
                , DateFormat.text " "
                , DateFormat.yearNumber
                , DateFormat.text " "
                , DateFormat.hourFixed
                , DateFormat.text ":"
                , DateFormat.minuteFixed
                , DateFormat.text ":"
                , DateFormat.secondFixed
                , DateFormat.text " "
                , DateFormat.amPmUppercase
                ]
                Time.utc
                -- TODO handle time zones
                date

        relativeDate =
            Duration.format ago ++ " ago"
    in
    if ago < 24 * 60 * 60 * 1000 then
        Html.tr []
            [ Html.td [ class "dict-key" ] [ Html.text label ]
            , Html.td
                [ title verboseDate, class "dict-value" ]
                [ Html.span [] [ Html.text relativeDate ] ]
            ]

    else
        Html.tr []
            [ Html.td [ class "dict-key" ] [ Html.text label ]
            , Html.td
                [ title relativeDate, class "dict-value" ]
                [ Html.span [] [ Html.text verboseDate ] ]
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
