module TooltipTests exposing (all)

import Browser.Dom
import Common
import Expect
import HoverState
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Test exposing (Test, describe, test)
import Tooltip


all : Test
all =
    describe ".handleCallback"
        [ describe "OnlyShowWhenOverflowing policy"
            [ test "callback with overflowing viewport turns hover -> pending" <|
                \_ ->
                    ( { hovered =
                            HoverState.Hovered <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                      }
                    , []
                    )
                        |> Tooltip.handleCallback
                            (Callback.GotViewport Callback.OnlyShowWhenOverflowing <|
                                Ok overflowingViewport
                            )
                        |> Tuple.first
                        |> .hovered
                        |> Expect.equal
                            (HoverState.TooltipPending
                                (SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                                )
                            )
            , test "callback with overflowing viewport gets element position" <|
                \_ ->
                    ( { hovered =
                            HoverState.Hovered <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                      }
                    , []
                    )
                        |> Tooltip.handleCallback
                            (Callback.GotViewport Callback.OnlyShowWhenOverflowing <|
                                Ok overflowingViewport
                            )
                        |> Tuple.second
                        |> Common.contains (Effects.GetElement domID)
            , test "callback with non-overflowing does nothing" <|
                \_ ->
                    ( { hovered =
                            HoverState.Hovered <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                      }
                    , []
                    )
                        |> Tooltip.handleCallback
                            (Callback.GotViewport Callback.OnlyShowWhenOverflowing <|
                                Ok nonOverflowingViewport
                            )
                        |> Tuple.first
                        |> .hovered
                        |> Expect.equal
                            (HoverState.Hovered <|
                                SideBarPipeline
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                            )
            ]
        , test "AlwaysShow callback with non-overflowing viewport gets element" <|
            \_ ->
                ( { hovered =
                        HoverState.Hovered <|
                            SideBarPipeline
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                  }
                , []
                )
                    |> Tooltip.handleCallback
                        (Callback.GotViewport Callback.AlwaysShow <|
                            Ok nonOverflowingViewport
                        )
                    |> Tuple.second
                    |> Common.contains (Effects.GetElement domID)
        , test "callback with tooltip position turns pending -> tooltip" <|
            \_ ->
                ( { hovered =
                        HoverState.TooltipPending
                            (SideBarPipeline
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                            )
                  }
                , []
                )
                    |> Tooltip.handleCallback
                        (Callback.GotElement <| Ok elementPosition)
                    |> Tuple.first
                    |> .hovered
                    |> Expect.equal
                        (HoverState.Tooltip
                            (SideBarPipeline
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                            )
                            { left = 1
                            , top = 0.5
                            , arrowSize = 15
                            , marginTop = -15
                            }
                        )
        ]


elementPosition : Browser.Dom.Element
elementPosition =
    { scene =
        { width = 0
        , height = 0
        }
    , viewport =
        { width = 0
        , height = 0
        , x = 0
        , y = 0
        }
    , element =
        { x = 0
        , y = 0
        , width = 1
        , height = 1
        }
    }


nonOverflowingViewport : Browser.Dom.Viewport
nonOverflowingViewport =
    { scene =
        { width = 1
        , height = 0
        }
    , viewport =
        { width = 1
        , height = 0
        , x = 0
        , y = 0
        }
    }


domID : DomID
domID =
    SideBarPipeline
        { teamName = "team"
        , pipelineName = "pipeline"
        }


overflowingViewport : Browser.Dom.Viewport
overflowingViewport =
    { scene =
        { width = 1
        , height = 0
        }
    , viewport =
        { width = 0
        , height = 0
        , x = 0
        , y = 0
        }
    }
