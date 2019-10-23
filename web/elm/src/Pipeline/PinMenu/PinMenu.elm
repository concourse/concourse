module Pipeline.PinMenu.PinMenu exposing (viewPinMenu)

import Concourse
import Dict
import Html exposing (Html)
import Html.Attributes exposing (id)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import Pipeline.Styles as Styles
import Routes


viewPinMenu :
    { pinnedResources : List ( String, Concourse.Version )
    , pipeline : Concourse.PipelineIdentifier
    , isPinMenuExpanded : Bool
    }
    -> Html Message
viewPinMenu ({ pinnedResources, isPinMenuExpanded } as params) =
    Html.div
        (id "pin-icon" :: Styles.pinIconContainer isPinMenuExpanded)
        [ if List.length pinnedResources > 0 then
            Html.div
                ([ onMouseEnter <| Hover <| Just PinIcon
                 , onMouseLeave <| Hover Nothing
                 ]
                    ++ Styles.pinIcon True
                )
                (Html.div
                    (id "pin-badge" :: Styles.pinBadge)
                    [ Html.div []
                        [ Html.text <|
                            String.fromInt <|
                                List.length pinnedResources
                        ]
                    ]
                    :: viewPinMenuDropdown params
                )

          else
            Html.div (Styles.pinIcon False) []
        ]


viewPinMenuDropdown :
    { pinnedResources : List ( String, Concourse.Version )
    , pipeline : Concourse.PipelineIdentifier
    , isPinMenuExpanded : Bool
    }
    -> List (Html Message)
viewPinMenuDropdown { pinnedResources, pipeline, isPinMenuExpanded } =
    if isPinMenuExpanded then
        [ Html.ul
            Styles.pinIconDropdown
            (pinnedResources
                |> List.map
                    (\( resourceName, pinnedVersion ) ->
                        Html.li
                            (onClick
                                (GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = pipeline.teamName
                                            , pipelineName = pipeline.pipelineName
                                            , resourceName = resourceName
                                            }
                                        , page = Nothing
                                        }
                                )
                                :: Styles.pinDropdownCursor
                            )
                            [ Html.div
                                Styles.pinText
                                [ Html.text resourceName ]
                            , Html.table []
                                (pinnedVersion
                                    |> Dict.toList
                                    |> List.map
                                        (\( k, v ) ->
                                            Html.tr []
                                                [ Html.td [] [ Html.text k ]
                                                , Html.td [] [ Html.text v ]
                                                ]
                                        )
                                )
                            ]
                    )
            )
        , Html.div Styles.pinHoverHighlight []
        ]

    else
        []
